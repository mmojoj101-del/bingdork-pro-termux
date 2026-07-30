// Package useragent provides a large pool of realistic, current browser
// User-Agent strings for anti-bot evasion. Supports Chrome, Firefox, Edge,
// Safari on desktop and mobile platforms with randomized version selection.
package useragent

import (
	"math/rand"
	"strings"
)

// Get returns a random User-Agent string from the pool.
func Get() string {
	return GetModern()
}

// GetModern returns a modern, current browser User-Agent.
func GetModern() string {
	category := rand.Intn(100)
	switch {
	case category < 45: // Chrome (45%)
		return randomChrome()
	case category < 65: // Firefox (20%)
		return randomFirefox()
	case category < 80: // Edge (15%)
		return randomEdge()
	case category < 90: // Safari Desktop (10%)
		return randomSafari()
	default: // Mobile (10%)
		return randomMobile()
	}
}

// GetChrome returns a Chrome User-Agent.
func GetChrome() string {
	return randomChrome()
}

// GetFirefox returns a Firefox User-Agent.
func GetFirefox() string {
	return randomFirefox()
}

// GetEdge returns an Edge (Chromium) User-Agent.
func GetEdge() string {
	return randomEdge()
}

// GetSafari returns a Safari User-Agent.
func GetSafari() string {
	return randomSafari()
}

// GetMobile returns a mobile User-Agent.
func GetMobile() string {
	return randomMobile()
}

// GetByCategory returns a UA from a specific category.
func GetByCategory(cat string) string {
	switch strings.ToLower(cat) {
	case "chrome":
		return randomChrome()
	case "firefox":
		return randomFirefox()
	case "edge":
		return randomEdge()
	case "safari":
		return randomSafari()
	case "mobile":
		return randomMobile()
	default:
		return GetModern()
	}
}

// ======== Chrome ========

var chromeVersions = []string{
	"124.0.6367.91", "124.0.6367.92", "124.0.6367.118", "124.0.6367.119",
	"125.0.6422.26", "125.0.6422.60", "125.0.6422.72", "125.0.6422.76",
	"125.0.6422.80", "125.0.6422.112", "125.0.6422.113", "125.0.6422.141",
	"126.0.6478.14", "126.0.6478.36", "126.0.6478.56", "126.0.6478.61",
	"126.0.6478.62", "126.0.6478.114", "126.0.6478.126", "126.0.6478.127",
	"126.0.6478.182", "126.0.6478.183",
	"127.0.6533.16", "127.0.6533.48", "127.0.6533.72", "127.0.6533.84",
	"127.0.6533.88", "127.0.6533.89", "127.0.6533.99",
	"128.0.6559.0", "128.0.6563.0", "128.0.6579.0", "128.0.6583.0",
	"129.0.6607.0", "129.0.6619.0",
}

var chromePlatforms = []string{
	"Windows NT 10.0; Win64; x64",
	"Windows NT 10.0; WOW64",
	"Windows NT 11.0; Win64; x64",
	"Macintosh; Intel Mac OS X 10_15_7",
	"Macintosh; Intel Mac OS X 14_5",
	"Macintosh; Intel Mac OS X 14_6",
	"Macintosh; Intel Mac OS X 13_6",
	"X11; Linux x86_64",
	"X11; Ubuntu; Linux x86_64",
	"X11; Fedora; Linux x86_64",
	"X11; Linux x86_64; rv:125.0",
}

func randomChrome() string {
	ver := chromeVersions[rand.Intn(len(chromeVersions))]
	platform := chromePlatforms[rand.Intn(len(chromePlatforms))]
	webkit := pickWebkitForChrome(ver)

	return "Mozilla/5.0 (" + platform + ") AppleWebKit/" + webkit + " (KHTML, like Gecko) Chrome/" + ver + " Safari/" + webkit
}

func pickWebkitForChrome(chromeVer string) string {
	// Extract major version
	parts := strings.Split(chromeVer, ".")
	if len(parts) > 0 {
		// Chrome 124+ uses WebKit 537.36
		return "537.36"
	}
	return "537.36"
}

// ======== Firefox ========

var firefoxVersions = []string{
	"124.0", "124.0.1", "124.0.2",
	"125.0", "125.0.1", "125.0.2", "125.0.3",
	"126.0", "126.0.1", "126.0.2",
	"127.0", "127.0.1", "127.0.2",
	"128.0", "128.0.1", "128.0.2", "128.0.3",
	"129.0", "129.0.1",
	"130.0",
	"115.10.0esr", "115.11.0esr", "115.12.0esr", "115.13.0esr",
	"115.14.0esr", "115.15.0esr",
}

var firefoxPlatforms = []string{
	"Windows NT 10.0; Win64; x64; rv:124.0",
	"Windows NT 10.0; Win64; x64; rv:125.0",
	"Windows NT 10.0; Win64; x64; rv:126.0",
	"Windows NT 10.0; Win64; x64; rv:127.0",
	"Windows NT 10.0; Win64; x64; rv:128.0",
	"Windows NT 10.0; Win64; x64; rv:129.0",
	"Windows NT 11.0; Win64; x64; rv:128.0",
	"Macintosh; Intel Mac OS X 10.15; rv:127.0",
	"Macintosh; Intel Mac OS X 10.15; rv:128.0",
	"Macintosh; Intel Mac OS X 14.5; rv:128.0",
	"Macintosh; Intel Mac OS X 14.6; rv:129.0",
	"X11; Linux x86_64; rv:128.0",
	"X11; Ubuntu; Linux x86_64; rv:127.0",
	"X11; Fedora; Linux x86_64; rv:128.0",
}

func randomFirefox() string {
	ver := firefoxVersions[rand.Intn(len(firefoxVersions))]
	rvPlatform := firefoxPlatforms[rand.Intn(len(firefoxPlatforms))]

	return "Mozilla/5.0 (" + rvPlatform + ") Gecko/20100101 Firefox/" + ver
}

// ======== Edge ========

var edgeVersions = []string{
	"124.0.2478.51", "124.0.2478.67", "124.0.2478.80", "124.0.2478.97",
	"124.0.2478.105",
	"125.0.2535.51", "125.0.2535.67", "125.0.2535.85", "125.0.2535.92",
	"125.0.2535.102",
	"126.0.2592.56", "126.0.2592.61", "126.0.2592.68", "126.0.2592.81",
	"126.0.2592.87", "126.0.2592.102", "126.0.2592.113",
	"127.0.2651.72", "127.0.2651.74", "127.0.2651.86", "127.0.2651.98",
	"127.0.2651.105",
	"128.0.2739.0", "128.0.2752.0",
}

var edgePlatforms = []string{
	"Windows NT 10.0; Win64; x64",
	"Windows NT 10.0; WOW64",
	"Windows NT 11.0; Win64; x64",
	"Macintosh; Intel Mac OS X 10_15_7",
	"Macintosh; Intel Mac OS X 14_5",
	"Macintosh; Intel Mac OS X 14_6",
	"X11; Linux x86_64",
}

func randomEdge() string {
	ver := edgeVersions[rand.Intn(len(edgeVersions))]
	platform := edgePlatforms[rand.Intn(len(edgePlatforms))]
	chromeVer := extractChromeFromEdge(ver)

	return "Mozilla/5.0 (" + platform + ") AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + chromeVer + " Safari/537.36 Edg/" + ver
}

func extractChromeFromEdge(edgeVer string) string {
	// Edge 124.x => Chrome 124.x
	parts := strings.Split(edgeVer, ".")
	if len(parts) > 0 {
		return parts[0] + ".0.6167.0"
	}
	return "124.0.6167.0"
}

// ======== Safari ========

var safariDesktopVersions = []string{
	"15.6", "15.6.1", "16.0", "16.1", "16.2", "16.3", "16.4", "16.5", "16.6",
	"17.0", "17.1", "17.2", "17.3", "17.4", "17.4.1", "17.5", "17.5.1",
	"17.6", "17.6.1",
	"18.0",
}

var safariMacPlatforms = []string{
	"Macintosh; Intel Mac OS X 10_15_7",
	"Macintosh; Intel Mac OS X 11_7_10",
	"Macintosh; Intel Mac OS X 12_7_5",
	"Macintosh; Intel Mac OS X 13_6",
	"Macintosh; Intel Mac OS X 14_5",
	"Macintosh; Intel Mac OS X 14_6",
	"Macintosh; Intel Mac OS X 15_0",
	"Macintosh; Intel Mac OS X 15_1",
}

func randomSafari() string {
	ver := safariDesktopVersions[rand.Intn(len(safariDesktopVersions))]
	platform := safariMacPlatforms[rand.Intn(len(safariMacPlatforms))]

	// Determine webkit version based on Safari version
	webkit := safariVersionToWebKit(ver)

	return "Mozilla/5.0 (" + platform + ") AppleWebKit/" + webkit + " (KHTML, like Gecko) Version/" + ver + " Safari/" + webkit
}

func safariVersionToWebKit(safariVer string) string {
	switch {
	case strings.HasPrefix(safariVer, "18."):
		return "618.1.15"
	case strings.HasPrefix(safariVer, "17.6"):
		return "617.1.17"
	case strings.HasPrefix(safariVer, "17.5"):
		return "617.1.15"
	case strings.HasPrefix(safariVer, "17.4"):
		return "617.1.11"
	case strings.HasPrefix(safariVer, "17.3"):
		return "617.1.9"
	case strings.HasPrefix(safariVer, "17.2"):
		return "617.1.7"
	case strings.HasPrefix(safariVer, "17.1"):
		return "617.1.5"
	case strings.HasPrefix(safariVer, "17.0"):
		return "617.1.3"
	case strings.HasPrefix(safariVer, "16.6"):
		return "616.1.22"
	case strings.HasPrefix(safariVer, "16.5"):
		return "616.1.20"
	case strings.HasPrefix(safariVer, "16.4"):
		return "616.1.18"
	case strings.HasPrefix(safariVer, "16.3"):
		return "616.1.15"
	case strings.HasPrefix(safariVer, "16.2"):
		return "616.1.13"
	case strings.HasPrefix(safariVer, "16.1"):
		return "616.1.10"
	case strings.HasPrefix(safariVer, "16.0"):
		return "616.1.7"
	default:
		return "605.1.15"
	}
}

// ======== Mobile ========

var mobileUAs = []string{
	// Android Chrome
	"Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.6422.72 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.6422.72 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Pixel 7 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.6422.113 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Samsung Galaxy S24 Ultra) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Samsung Galaxy S24+) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Samsung Galaxy S24) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Samsung Galaxy S23 Ultra) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Samsung Galaxy S23) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; OnePlus 12) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Xiaomi 14 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.6422.72 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Xiaomi 13T) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; Samsung Galaxy S22) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; OnePlus 11) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.6422.72 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; Google Pixel 6a) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; Motorola Edge 30) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 Mobile Safari/537.36",

	// Android Samsung Browser
	"Mozilla/5.0 (Linux; Android 14; SAMSUNG SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/124.0.6367.82 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; SAMSUNG SM-S926B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/24.0 Chrome/122.0.6261.105 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; SAMSUNG SM-S921B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/124.0.6367.82 Mobile Safari/537.36",

	// Android Firefox
	"Mozilla/5.0 (Android 14; Mobile; rv:128.0) Gecko/128.0 Firefox/128.0",
	"Mozilla/5.0 (Android 14; Mobile; rv:127.0) Gecko/127.0 Firefox/127.0",
	"Mozilla/5.0 (Android 13; Mobile; rv:126.0) Gecko/126.0 Firefox/126.0",

	// iOS Safari
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_3_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 16_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1",

	// iOS Chrome
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/126.0.6478.54 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/126.0.6478.54 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/125.0.6422.51 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/128.0.6613.0 Mobile/15E148 Safari/604.1",

	// iOS Firefox
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/128.0 Mobile/15E148 Safari/605.1.15",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/127.0 Mobile/15E148 Safari/605.1.15",

	// iPad
	"Mozilla/5.0 (iPad; CPU OS 17_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
}

func randomMobile() string {
	return mobileUAs[rand.Intn(len(mobileUAs))]
}

// ======== Additional Legacy/Common UAs ========

var commonUAs = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.127 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.6422.141 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14.6; rv:128.0) Gecko/20100101 Firefox/128.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.2592.113 Safari/537.36 Edg/126.0.2592.113",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.127 Safari/537.36",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0",
}
