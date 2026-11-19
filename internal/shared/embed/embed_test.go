package embed

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	ok := func(provider, id string) Embed { return Embed{Provider: provider, ExternalID: id} }

	tests := []struct {
		in   string
		want Embed // zero value = must fail
	}{
		// YouTube
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", ok(YouTube, "dQw4w9WgXcQ")},
		{"https://youtube.com/watch?v=dQw4w9WgXcQ&t=42s&list=PL123", ok(YouTube, "dQw4w9WgXcQ")},
		{"https://m.youtube.com/watch?v=dQw4w9WgXcQ", ok(YouTube, "dQw4w9WgXcQ")},
		{"https://youtu.be/dQw4w9WgXcQ?si=AbCdEf123", ok(YouTube, "dQw4w9WgXcQ")},
		{"youtu.be/dQw4w9WgXcQ", ok(YouTube, "dQw4w9WgXcQ")}, // no scheme
		{"  https://YOUTUBE.com/shorts/dQw4w9WgXcQ  ", ok(YouTube, "dQw4w9WgXcQ")},
		{"https://www.youtube.com/embed/dQw4w9WgXcQ", ok(YouTube, "dQw4w9WgXcQ")},
		{"https://www.youtube.com/live/dQw4w9WgXcQ?feature=share", ok(YouTube, "dQw4w9WgXcQ")},
		{"https://www.youtube.com:443/watch?v=dQw4w9WgXcQ", ok(YouTube, "dQw4w9WgXcQ")},

		// Vimeo
		{"https://vimeo.com/76979871", ok(Vimeo, "76979871")},
		{"https://vimeo.com/76979871/8272103f6e", ok(Vimeo, "76979871:8272103f6e")},
		{"https://player.vimeo.com/video/76979871?h=8272103f6e", ok(Vimeo, "76979871:8272103f6e")},
		{"https://vimeo.com/channels/staffpicks/76979871", ok(Vimeo, "76979871")},

		// TikTok
		{"https://www.tiktok.com/@scout2015/video/6718335390845095173", ok(TikTok, "6718335390845095173")},
		{"https://www.tiktok.com/@scout2015/video/6718335390845095173?is_from_webapp=1", ok(TikTok, "6718335390845095173")},

		// Twitch
		{"https://www.twitch.tv/videos/1234567890", ok(TwitchVideo, "1234567890")},
		{"https://clips.twitch.tv/AwkwardHelplessSalamanderSwiftRage-abc_12", ok(TwitchClip, "AwkwardHelplessSalamanderSwiftRage-abc_12")},
		{"https://www.twitch.tv/somechannel/clip/AwkwardHelplessSalamander", ok(TwitchClip, "AwkwardHelplessSalamander")},

		// must fail
		{"", Embed{}},
		{"https://evil.com/watch?v=dQw4w9WgXcQ", Embed{}},
		{"https://youtube.com.evil.com/watch?v=dQw4w9WgXcQ", Embed{}},
		{"https://evilyoutube.com/watch?v=dQw4w9WgXcQ", Embed{}},
		{"javascript:alert(1)", Embed{}},
		{"ftp://youtube.com/watch?v=dQw4w9WgXcQ", Embed{}},
		{"https://youtube.com/watch?v=short", Embed{}},
		{"https://youtube.com/watch?v=dQw4w9WgXcQ<script>", Embed{}},
		{"https://youtube.com/watch", Embed{}},
		{"https://youtube.com/channel/UCxyz", Embed{}},
		{"https://vimeo.com/76979871/NOT-A-HASH", Embed{}},
		{"https://vimeo.com/abc", Embed{}},
		{"https://vm.tiktok.com/ZMabc123/", Embed{}}, // short link: would need a server-side redirect (SSRF)
		{"https://www.tiktok.com/@user", Embed{}},
		{"https://twitch.tv/somechannel", Embed{}},
	}

	for _, tt := range tests {
		got, err := Parse(tt.in)
		if tt.want == (Embed{}) {
			if !errors.Is(err, ErrUnsupportedEmbed) {
				t.Errorf("Parse(%q) = %+v, %v; want ErrUnsupportedEmbed", tt.in, got, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", tt.in, got, err, tt.want)
		}
	}
}
