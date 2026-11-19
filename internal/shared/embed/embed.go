// Package embed turns a user-pasted video link (YouTube, Vimeo, TikTok, Twitch) into a
// (provider, id) pair.
//
// SECURITY: the raw link is never stored or rendered. Only the provider and an id that
// matched a strict pattern are kept; the frontend builds the iframe URL from its own
// template per provider, so whatever a user pastes, the iframe can only point at that
// provider's player.
package embed

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// ErrUnsupportedEmbed: not a link to a supported provider, or the video id is malformed.
var ErrUnsupportedEmbed = errors.New("unsupported embed link")

const (
	YouTube     = "youtube"
	Vimeo       = "vimeo"
	TikTok      = "tiktok"
	TwitchVideo = "twitch_video"
	TwitchClip  = "twitch_clip"
)

// Providers lists every provider value Parse can return (the DB CHECK constraint must match).
var Providers = []string{YouTube, Vimeo, TikTok, TwitchVideo, TwitchClip}

const maxLinkLen = 2048

// ^...$ everywhere: MatchString matches anywhere in the string without anchors.
var (
	youtubeID  = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	vimeoID    = regexp.MustCompile(`^[0-9]{1,12}$`)
	vimeoHash  = regexp.MustCompile(`^[0-9a-f]{6,20}$`)
	tiktokID   = regexp.MustCompile(`^[0-9]{15,20}$`)
	twitchID   = regexp.MustCompile(`^[0-9]{1,12}$`)
	twitchSlug = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
)

// Embed is what gets stored. ExternalID is the provider's video id; for unlisted Vimeo
// videos it is "ID:HASH" (the hash is required to play them).
type Embed struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"id"`
}

// Parse extracts the provider and video id from a link. Links without a scheme
// ("youtu.be/xyz") are accepted. Short redirect links (vm.tiktok.com, ...) are not:
// resolving them would mean the server fetching a user-supplied URL (SSRF).
func Parse(raw string) (Embed, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxLinkLen {
		return Embed{}, ErrUnsupportedEmbed
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return Embed{}, ErrUnsupportedEmbed
	}

	host := strings.ToLower(u.Hostname()) // Hostname drops the port
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")

	// "/watch/" -> ["watch"]; "/" -> [""]
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")

	// exact host match only: Contains / HasSuffix would let "youtube.com.evil.com" through
	switch host {
	case "youtube.com", "youtube-nocookie.com":
		return parseYouTube(u, parts)
	case "youtu.be":
		return match(YouTube, parts[0], youtubeID)
	case "vimeo.com", "player.vimeo.com":
		return parseVimeo(u, host, parts)
	case "tiktok.com":
		// /@user/video/ID
		if len(parts) == 3 && strings.HasPrefix(parts[0], "@") && parts[1] == "video" {
			return match(TikTok, parts[2], tiktokID)
		}
	case "twitch.tv":
		// /videos/ID or /<channel>/clip/SLUG
		if len(parts) == 2 && parts[0] == "videos" {
			return match(TwitchVideo, parts[1], twitchID)
		}
		if len(parts) == 3 && parts[1] == "clip" {
			return match(TwitchClip, parts[2], twitchSlug)
		}
	case "clips.twitch.tv":
		if len(parts) == 1 {
			return match(TwitchClip, parts[0], twitchSlug)
		}
	}
	return Embed{}, ErrUnsupportedEmbed
}

func parseYouTube(u *url.URL, parts []string) (Embed, error) {
	switch {
	case len(parts) == 1 && parts[0] == "watch":
		return match(YouTube, u.Query().Get("v"), youtubeID)
	case len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live"):
		return match(YouTube, parts[1], youtubeID)
	}
	return Embed{}, ErrUnsupportedEmbed
}

func parseVimeo(u *url.URL, host string, parts []string) (Embed, error) {
	var id, hash string
	switch {
	case host == "player.vimeo.com" && len(parts) == 2 && parts[0] == "video":
		id, hash = parts[1], u.Query().Get("h") // player.vimeo.com/video/ID?h=HASH
	case len(parts) == 1:
		id = parts[0] // vimeo.com/ID
	case len(parts) == 2 && vimeoID.MatchString(parts[0]):
		id, hash = parts[0], parts[1] // vimeo.com/ID/HASH (unlisted)
	case len(parts) == 3 && parts[0] == "channels":
		id = parts[2] // vimeo.com/channels/NAME/ID
	default:
		return Embed{}, ErrUnsupportedEmbed
	}

	if !vimeoID.MatchString(id) {
		return Embed{}, ErrUnsupportedEmbed
	}
	if hash == "" {
		return Embed{Provider: Vimeo, ExternalID: id}, nil
	}
	if !vimeoHash.MatchString(hash) {
		return Embed{}, ErrUnsupportedEmbed
	}
	return Embed{Provider: Vimeo, ExternalID: id + ":" + hash}, nil
}

func match(provider, id string, re *regexp.Regexp) (Embed, error) {
	if !re.MatchString(id) {
		return Embed{}, ErrUnsupportedEmbed
	}
	return Embed{Provider: provider, ExternalID: id}, nil
}
