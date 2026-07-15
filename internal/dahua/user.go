package dahua

import "fmt"

// SetPassword changes the logged-in account's password via
// userManager.modifyPassword, using the credential the client logged in with as
// the old password. The exact hashing accepted varies by firmware; on rejection
// the device error is returned so the caller can surface it in the live log.
func (c *Client) SetPassword(newPass string) error {
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
	return nil
}
