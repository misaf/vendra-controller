package urls

func Platform(base string) map[string]string {
	return map[string]string{"website": "https://" + base, "admin": "https://admin." + base, "console": "https://console." + base, "reseller": "https://reseller." + base, "api": "https://api." + base + "/up", "dashboard": "http://127.0.0.1:8080/dashboard/"}
}
