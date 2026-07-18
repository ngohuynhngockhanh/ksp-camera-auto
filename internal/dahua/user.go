package dahua

import "fmt"

// passwordHash is the Dahua "gen2" stored-password hash,
// UPPER(MD5(user:realm:password)) — the form modern firmware's
// userManager.modifyPassword expects for pwd/pwdOld (not plaintext). It's the
// same inner hash gen2Hash computes during login (see hash.go).
func passwordHash(user, realm, pass string) string {
	return md5Upper(user + ":" + realm + ":" + pass)
}

// SetPassword changes the logged-in account's password via
// userManager.modifyPassword, using the credential the client logged in with
// as the old password. Firmware disagrees on the wire form: modern devices
// require pwd/pwdOld as the gen2 hash UPPER(MD5(user:realm:pass)) with
// authorityType "Default", while older ones take plaintext. We try the hashed
// form first (what current firmware needs — plaintext just gets rejected
// there) and fall back to plaintext, so both generations work. On rejection
// the device error is returned so the caller can surface it in the live log.
func (c *Client) SetPassword(newPass string) error {
	// Preferred: hashed form for modern firmware.
	if c.realm != "" {
		resp, err := c.Call("userManager.modifyPassword", map[string]any{
			"name":          c.user,
			"pwd":           passwordHash(c.user, c.realm, newPass),
			"pwdOld":        passwordHash(c.user, c.realm, c.pass),
			"authorityType": "Default",
		})
		if err != nil {
			return err
		}
		if resp.ok() {
			c.pass = newPass // keep the session's remembered credential in sync
			return nil
		}
		hashedErr := respErr(resp)

		// Fall back to the legacy plaintext form for older firmware.
		resp2, err := c.Call("userManager.modifyPassword", map[string]any{
			"name":   c.user,
			"pwd":    newPass,
			"pwdOld": c.pass,
		})
		if err != nil {
			return err
		}
		if resp2.ok() {
			c.pass = newPass
			return nil
		}
		return fmt.Errorf("modifyPassword: %s (hashed form: %s)", respErr(resp2), hashedErr)
	}

	// No realm captured (shouldn't happen after a normal login) — plaintext only.
	resp, err := c.Call("userManager.modifyPassword", map[string]any{
		"name":   c.user,
		"pwd":    newPass,
		"pwdOld": c.pass,
	})
	if err != nil {
		return err
	}
	if !resp.ok() {
		return fmt.Errorf("modifyPassword: %s", respErr(resp))
	}
	c.pass = newPass
	return nil
}
