# Texman

Texman is a terminal HTTP client for working with request collections stored as JSON. It lets you browse collections, run requests, edit methods, URLs, headers and bodies, create or delete requests, import/export collections, and review saved responses.

The interface is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), so it runs inside an interactive terminal.

## Install

Every push to `main` publishes two binaries on the [latest GitHub release](https://github.com/LuisTejedaS/texman/releases/latest):

| File | System |
| --- | --- |
| `texman-macos-arm64` | macOS on Apple Silicon |
| `texman-windows-amd64.exe` | 64-bit Windows |

Start Texman from the folder where its data should live. It reads request collections from `./collections` and writes saved responses to `./responses`. Those folders appear when you save a collection or run a request.

### macOS

The release binary targets Apple Silicon (`arm64`). In Terminal:

```bash
curl -fsSL -o texman https://github.com/LuisTejedaS/texman/releases/latest/download/texman-macos-arm64
chmod +x texman
xattr -d com.apple.quarantine texman
./texman
```

`xattr` clears the quarantine flag macOS adds to files downloaded from the internet. If Gatekeeper still blocks the app, open **System Settings → Privacy & Security** and choose **Open Anyway**.

To run `texman` from any directory, move it onto your `PATH`:

```bash
sudo mv texman /usr/local/bin/texman
```

Then change to the folder that should hold `collections` and `responses`, and start it:

```bash
cd /path/to/your/texman-data
texman
```

Quit with `q` or `ctrl+c`.

### Windows

The release binary is a 64-bit executable. In PowerShell:

```powershell
Invoke-WebRequest -Uri "https://github.com/LuisTejedaS/texman/releases/latest/download/texman-windows-amd64.exe" -OutFile ".\texman.exe"
.\texman.exe
```

Start it from PowerShell, Command Prompt, or Windows Terminal. SmartScreen may report an unknown publisher because the file is unsigned. Choose **More info**, then **Run anyway**.

To run `texman` from any folder, put `texman.exe` in a directory already on your `PATH`, or add its folder to `PATH`. Always launch it from the directory where `collections` and `responses` should live:

```powershell
cd C:\path\to\your\texman-data
texman.exe
```

Quit with `q` or `ctrl+c`.

## Requirements

Building from source needs:

- Go 1.25 or later
- Internet access the first time dependencies are downloaded

## Run In Development

```bash
go run .
```

Texman reads collections from `./collections` and saves responses to `./responses`, so it is best to run it from the repository root.

## Build

To build a local binary:

```bash
go build -o texman .
```

Then run it with:

```bash
./texman
```

To build a Windows executable:

```bash
GOOS=windows GOARCH=amd64 go build -o texman.exe .
```

## Usage

When the app opens, the screen is split into:

- `Collections`: the list of collections and requests.
- `Request`: details for the selected request.
- `Response`: the result of the last executed request.

### General Navigation

| Key | Action |
| --- | --- |
| `j` / `down` | Move selection down |
| `k` / `up` | Move selection up |
| `tab` | Switch focus between the sidebar and detail panel |
| `enter` | Run the selected request or expand/collapse a collection |
| `space` | Expand/collapse a collection |
| `r` | Run a request; when a collection is selected, rename it |
| `q` / `ctrl+c` | Quit |

When the response panel has focus:

| Key | Action |
| --- | --- |
| `j` / `down` | Scroll down |
| `k` / `up` | Scroll up |
| `f` / `pgdown` | Page down |
| `b` / `pgup` | Page up |

### Editing Requests

Select a request and use:

| Key | Action |
| --- | --- |
| `n` | Create a new request in the current collection |
| `m` | Edit the HTTP method |
| `u` | Edit the URL |
| `h` | Edit headers |
| `e` | Edit the body |
| `d` | Delete the selected request |

Inside editors:

| Key | Action |
| --- | --- |
| `enter` | Confirm the current step or save simple fields |
| `ctrl+s` | Save changes |
| `esc` | Cancel or go back to the previous step |

In the header editor:

| Key | Action |
| --- | --- |
| `enter` | Edit the selected header |
| `a` | Add a header |
| `d` | Delete a header |
| `ctrl+s` | Save all headers |

### Collections

| Key | Action |
| --- | --- |
| `C` | Create a collection |
| `r` | Rename the selected collection |
| `D` | Delete the selected collection |
| `i` | Import a collection from a JSON file |
| `x` | Export the selected collection to a JSON file |

### Saved Responses

Every executed request automatically saves a response in `./responses` using this format:

```text
YYYY-MM-DD_HH-MM-SS_request-name.txt
```

To view saved responses:

| Key | Action |
| --- | --- |
| `v` | Switch between collections and saved responses |
| `enter` | Open the selected response |
| `d` | Delete the selected response file |

## Collection Format

Each `*.json` file inside `collections` represents one collection:

```json
{
  "name": "st-location-service",
  "requests": [
    {
      "name": "ip-info",
      "method": "GET",
      "url": "https://example.com/api/v1/ip-info",
      "headers": {
        "Authorization": "Bearer token"
      },
      "body": ""
    }
  ]
}
```

Request fields:

- `name`: the name shown in the list.
- `method`: the HTTP method, for example `GET`, `POST`, `PUT`, `PATCH`, or `DELETE`.
- `url`: the full endpoint URL.
- `headers`: a map of HTTP headers.
- `body`: the request body as text.

## Project Structure

```text
.
├── collections/          # HTTP collections in JSON
├── responses/            # Automatically saved responses
├── internal/
│   ├── collection/       # Collection loading and saving
│   ├── httpclient/       # HTTP request execution
│   ├── model/            # Bubble Tea state and logic
│   ├── responses/        # Response saving and listing
│   └── ui/               # Panel rendering and styles
├── main.go               # Entry point
├── go.mod
└── go.sum
```

## Notes

- The HTTP client uses a 30-second timeout.
- If the response is valid JSON, Texman displays it formatted.
- Collection changes are saved to the corresponding JSON file.
- Generated binaries, such as `texman` or `texman.exe`, are not required for development if you use `go run .`.
