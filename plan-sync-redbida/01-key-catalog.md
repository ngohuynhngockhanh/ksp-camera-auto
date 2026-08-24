# Key Catalog Policy

The catalog unions a reviewed baseline from `inut_204_63` with filenames from
the configured key directory, then applies semantic metadata in
`internal/redbida/catalog.go`. This lets a clean box such as `inut_204_163`
create approved branding keys while still discovering future keys read-only.

Groups:

- Branding / Logo
- Livestream
- UI / Display
- Schedule / Maintenance
- Network / MQTT
- Security / Credentials
- Advanced / Unknown

Risk policy:

- `editable`: explicitly allowlisted branding, livestream and safe UI values;
- `confirm-required`: explicitly allowlisted reboot/restart/schedule operations;
- `read-only-protected`: credentials, tokens, network, broker, identity and
  internal control files;
- `unknown`: visible for survey but rejected on submit.

Heuristics only choose a group; they never grant write access. Secret values
are masked in API responses. Values are read through `i_gets`, not by exposing
the contents of the remote directory. Writes fail closed if the key directory
cannot be inspected.
