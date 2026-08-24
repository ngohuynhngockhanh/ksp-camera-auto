# UI Form

The RedBida tab groups keys, supports search/filter, shows risk state and keeps
protected values read-only.

Logo keys support PNG/JPEG/WebP upload up to 512 KiB. The browser converts an
upload to a data URL; the backend validates it before sending the string through
`i_sets`. Existing URL/path values remain editable as text.

Submit shows a confirmation dialog whenever a `confirm-required` key is dirty.
The result message reports remote acknowledgement success or per-key errors.
