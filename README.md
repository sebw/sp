# Start Page

A start page with fuzzy search.

```
docker run -d --name sp --restart always -p 8194:8080 \
	--network=start \
	-v ./sp/data:/app/data \
	-v ./sp/icon_cache:/app/icon_cache \
	ghcr.io/sebw/sp:20260414
```

The `icon_cache` folder is served as `/icons/` internally.  
You can store images in that folder and add `/icons/path_to_your_image.png` as icon.
