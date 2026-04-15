package main

import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

type Link struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	URL      string `json:"url"`
	Icon     string `json:"icon"` // New field
}

const csvFileName = "data/links.csv"
const cacheDir = "icon_cache"

func main() {
	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Fatal(err)
	}

	if _, err := os.Stat(csvFileName); os.IsNotExist(err) {
		createDefaultCSV()
	}

	r := mux.NewRouter()

	// API Routes
	r.HandleFunc("/api/links", getLinks).Methods("GET")
	r.HandleFunc("/api/links", addLink).Methods("POST")
	r.HandleFunc("/api/links/update", updateLink).Methods("POST") // New route
	r.HandleFunc("/api/links/delete", deleteLink).Methods("DELETE")

	// Serve cached icons
	r.PathPrefix("/icons/").Handler(http.StripPrefix("/icons/", http.FileServer(http.Dir(cacheDir))))

	// Static files
	r.PathPrefix("/").Handler(http.FileServer(http.Dir(".")))

	// Main Page
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("index.html"))
		tmpl.Execute(w, nil)
	})

	fmt.Println("Server starting at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// downloadAndCacheIcon downloads an image and saves it locally
func downloadAndCacheIcon(iconURL string) (string, error) {
	if iconURL == "" {
		return "", nil
	}

	// Generate a unique filename based on URL hash
	hash := md5.Sum([]byte(iconURL))
	filename := hex.EncodeToString(hash[:]) + filepath.Ext(iconURL)
	if filename == "." {
		filename = hex.EncodeToString(hash[:]) + ".png" // Default extension
	}

	localPath := filepath.Join(cacheDir, filename)

	// Check if already cached
	if _, err := os.Stat(localPath); err == nil {
		return "/icons/" + filename, nil
	}

	// Download
	resp, err := http.Get(iconURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download icon: %s", resp.Status)
	}

	// Save
	out, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		os.Remove(localPath) // Clean up on error
		return "", err
	}

	return "/icons/" + filename, nil
}

func generateID(title, category, url string) string {
	data := fmt.Sprintf("%s|%s|%s", title, category, url)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func createDefaultCSV() {
	file, err := os.Create(csvFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	// New format: Category;Title;URL;Icon
	writer.Write([]string{"Email", "Proton Mail", "https://proton.me/mail", "https://proton.me/favicon.ico"})
	writer.Write([]string{"Tools", "Fzf GitHub", "https://github.com/junegunn/fzf", "https://github.com/fluidicon.png"})
	writer.Write([]string{"Dev", "Go Docs", "https://pkg.go.dev", "https://go.dev/favicon.ico"})
}

func readCSV() ([]Link, error) {
	file, err := os.Open(csvFileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	// Allow variable number of fields (some might not have icons)
	reader.FieldsPerRecord = -1 

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var links []Link
	for _, record := range records {
		if len(record) < 3 {
			continue
		}
		
		category := record[0]
		title := record[1]
		url := record[2]
		icon := ""
		if len(record) >= 4 {
			icon = record[3]
		}

		id := generateID(title, category, url)

		links = append(links, Link{
			ID:       id,
			Title:    title,
			Category: category,
			URL:      url,
			Icon:     icon,
		})
	}
	return links, nil
}

func writeCSV(links []Link) error {
	file, err := os.Create(csvFileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	for _, l := range links {
		if l.Icon != "" {
			writer.Write([]string{l.Category, l.Title, l.URL, l.Icon})
		} else {
			writer.Write([]string{l.Category, l.Title, l.URL})
		}
	}
	return nil
}

func getLinks(w http.ResponseWriter, r *http.Request) {
	links, err := readCSV()
	if err != nil {
		log.Printf("Error reading CSV: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

func addLink(w http.ResponseWriter, r *http.Request) {
	var newLink Link
	if err := json.NewDecoder(r.Body).Decode(&newLink); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Cache the icon if provided
	if newLink.Icon != "" {
		cachedIcon, err := downloadAndCacheIcon(newLink.Icon)
		if err != nil {
			log.Printf("Warning: Could not cache icon for %s: %v", newLink.Title, err)
			// Continue anyway, just don't set the icon
			newLink.Icon = ""
		} else {
			newLink.Icon = cachedIcon
		}
	}

	newLink.ID = generateID(newLink.Title, newLink.Category, newLink.URL)

	links, err := readCSV()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check for duplicates
	for _, l := range links {
		if l.URL == newLink.URL {
			http.Error(w, "Link already exists", http.StatusConflict)
			return
		}
	}

	links = append(links, newLink)

	if err := writeCSV(links); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newLink)
}

func updateLink(w http.ResponseWriter, r *http.Request) {
	var updatedLink Link
	if err := json.NewDecoder(r.Body).Decode(&updatedLink); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Cache the icon if provided
	// if the icon has already been cached it starts with /icons/ and no attempt at recaching should be done otherwise it fails.
	if updatedLink.Icon != "" {
		if strings.HasPrefix(updatedLink.Icon, "http") {
			cachedIcon, err := downloadAndCacheIcon(updatedLink.Icon)
			if err != nil {
				log.Printf("Warning: Could not cache icon for %s: %v", updatedLink.Title, err)
				// Keep old icon if caching fails? Or clear it? Let's keep old if provided, else clear.
				// For now, if caching fails, we just don't update the icon field in the struct yet.
				// But we need to know the OLD icon to preserve it if the new one fails.
				// Simpler: If caching fails, we just don't set the new icon.
				updatedLink.Icon = "" 
			} else {
				updatedLink.Icon = cachedIcon
			}
		}
	}

	links, err := readCSV()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	found := false
	var newLinks []Link
	for _, l := range links {
		if l.ID == updatedLink.ID {
			// Update the link
			l.Title = updatedLink.Title
			l.Category = updatedLink.Category
			l.URL = updatedLink.URL
			l.Icon = updatedLink.Icon
			// Regenerate ID if Title/Category/URL changed? 
			// If we change the URL, the ID changes. This might break delete logic if we rely on ID.
			// But since we are updating the whole object, we should regenerate the ID if content changed.
			// However, the frontend sends the OLD ID.
			// If we change the URL, the ID changes. The next time we read, the ID will be different.
			// This is tricky. 
			// Solution: If URL changes, we must update the ID in the struct.
			// But the frontend uses ID for delete. If we change ID, the frontend's delete button (which has the old ID) will fail next time.
			// Better approach: Don't change the ID if only Title/Category/Icon changes.
			// If URL changes, the ID MUST change.
			// Let's regenerate ID only if URL changed.
			if l.URL != updatedLink.URL {
				l.ID = generateID(l.Title, l.Category, l.URL)
			}
			found = true
		}
		newLinks = append(newLinks, l)
	}

	if !found {
		http.Error(w, "Link not found", http.StatusNotFound)
		return
	}

	if err := writeCSV(newLinks); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedLink)
}

func deleteLink(w http.ResponseWriter, r *http.Request) {
	idToDelete := r.URL.Query().Get("id")
	if idToDelete == "" {
		http.Error(w, "Missing ID parameter", http.StatusBadRequest)
		return
	}

	links, err := readCSV()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var filtered []Link
	found := false
	for _, l := range links {
		if l.ID == idToDelete {
			found = true
			continue
		}
		filtered = append(filtered, l)
	}

	if !found {
		http.Error(w, "Link not found", http.StatusNotFound)
		return
	}

	if err := writeCSV(filtered); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
