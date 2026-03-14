# ui-kusumc (RMS admin UI)

Frontend for the KUSUMC RMS product line (within `rms-go/`).

## Local dev
```pwsh
cd rms-go\ui-kusumc\version-a-frontend
npm run dev
```

## Configure API base URL
- Default: same-origin `/api` (when served behind Nginx)
- Override: set `VITE_API_BASE_URL` (for example `https://localhost/api` or `http://localhost:8081/api`)

## Tests
```pwsh
cd rms-go\ui-kusumc\version-a-frontend
npm test
```
