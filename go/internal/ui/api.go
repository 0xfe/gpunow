package ui

type APICall struct {
	spinner *Spinner
}

func (u *UI) APICall(action, resource, label string) *APICall {
	if action != "" || resource != "" {
		if split := u.currentSplit(); split != nil && split.Active() {
			split.AppendAPI(u.formatAPILine(action, resource))
		} else {
			u.Detailf(1, "api: %s %s", action, resource)
		}
	}
	var sp *Spinner
	if label != "" && !(u.hasLive()) {
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

func (u *UI) formatAPILine(action, resource string) string {
	line := u.Indent(1, "-> api: "+action+" "+resource)
	return u.style(u.styles.Dim, line)
}
