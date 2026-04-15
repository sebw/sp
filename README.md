# Start Page

A very simple start page written in go with fuzzy search provided by `fuse.js`.

```
docker run -d --name sp --restart always -p 8194:8080 \
	--network=start \
	-v ./sp/data:/app/data \
	-v ./sp/icon_cache:/app/icon_cache \
	ghcr.io/sebw/sp:20260414
```

There's no database. Your links are stored in a CSV file.

Place your `links.csv` in your data folder.

The format of the CSV is this:

```
Category;Title;URL;optional_icon_path
```

The `icon_cache` folder is served as `/icons/` internally.

You can store images in that folder and add `/icons/path_to_your_image.png` as the path to the icon.

Alternatively you can edit your link from the web UI and specify a URL to the image. The image will be retrieved and cached locally and the path will be updated with the local path.

For performance reasons, try to keep icons under 20 kB.

You can extract favicons easily using this [online tool](https://onlineminitools.com/website-favicon-downloader).
