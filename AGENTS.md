# Project Notes

- Do not read audio or video file contents in this repository unless explicitly requested. Treat generated media files as opaque outputs; inspect scripts and text metadata instead.
- If the renderer starts using any new FFmpeg filter, also add that filter to the `requiredFilters` list used by `--check` in `main.go`.
