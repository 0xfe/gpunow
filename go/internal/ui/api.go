package ui

type APICall struct {
	spinner *Spinner
}

func (u *UI) APICall(action, resource, label string) *APICall {
	if action != "" || resource != "" {
		u.Detailf(1, "api: %s %s", action, resource)
	}
	var sp *Spinner
	if label != "" {
		sp = u.StartSpinner(label)
	}
	return &APICall{spinner: sp}
}

func (c *APICall) Stop() {
	if c == nil || c.spinner == nil {
		return
	}
	c.spinner.Stop()
}
