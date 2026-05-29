package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiBase = "https://api.spotify.com/v1"

type TokenProvider interface {
	AccessToken() (string, error)
}

type Client struct {
	tokens TokenProvider
	http   *http.Client
}

func New(tokens TokenProvider) *Client {
	return &Client{
		tokens: tokens,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) put(ctx context.Context, path string, body any) error {
	return c.do(ctx, http.MethodPut, path, body, nil)
}

func (c *Client) post(ctx context.Context, path string, body any) error {
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	token, err := c.tokens.AccessToken()
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = strings.NewReader(string(b))
	}

	target := apiBase + path
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		var apiErr struct {
			Error struct {
				Status  int    `json:"status"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &apiErr)
		if apiErr.Error.Message != "" {
			return fmt.Errorf("spotify API %d: %s", apiErr.Error.Status, apiErr.Error.Message)
		}
		return fmt.Errorf("spotify API %d: %s", resp.StatusCode, string(raw))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// ── Types ────────────────────────────────────────────────────────────────────

type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Album struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Images []Image `json:"images"`
}

type Track struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Artists  []Artist `json:"artists"`
	Album    Album    `json:"album"`
	Duration int      `json:"duration_ms"`
}

func (t Track) ArtistsString() string {
	names := make([]string, len(t.Artists))
	for i, a := range t.Artists {
		names[i] = a.Name
	}
	return strings.Join(names, ", ")
}

type Owner struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Playlist struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Images []Image `json:"images"`
	Owner  Owner   `json:"owner"`
	Public bool    `json:"public"`
	Collaborative bool `json:"collaborative"`
	Tracks struct {
		Total int `json:"total"`
	} `json:"tracks"`
}

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}


type Device struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	IsActive      bool   `json:"is_active"`
	VolumePercent int    `json:"volume_percent"`
}

type PlaybackState struct {
	IsPlaying  bool   `json:"is_playing"`
	ProgressMs int    `json:"progress_ms"`
	Item       Track  `json:"item"`
	Device     Device `json:"device"`
}

// ── API Methods ──────────────────────────────────────────────────────────────

func (c *Client) CurrentPlayback(ctx context.Context) (*PlaybackState, error) {
	var s PlaybackState
	if err := c.get(ctx, "/me/player", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Play starts playback. When contextURI is set (e.g. a playlist URI), trackURI
// becomes an offset into that context — this gives the player a real queue so
// next/previous work. Without a context, trackURI plays as a single one-off.
func (c *Client) Play(ctx context.Context, deviceID, contextURI, trackURI string) error {
	body := map[string]any{}
	switch {
	case contextURI != "":
		body["context_uri"] = contextURI
		if trackURI != "" {
			body["offset"] = map[string]any{"uri": trackURI}
		}
	case trackURI != "":
		body["uris"] = []string{trackURI}
	}
	path := "/me/player/play"
	if deviceID != "" {
		path += "?device_id=" + deviceID
	}
	return c.put(ctx, path, body)
}

func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	var result struct {
		Devices []Device `json:"devices"`
	}
	if err := c.get(ctx, "/me/player/devices", &result); err != nil {
		return nil, err
	}
	return result.Devices, nil
}

func (c *Client) TransferPlayback(ctx context.Context, deviceID string) error {
	return c.put(ctx, "/me/player", map[string]any{
		"device_ids": []string{deviceID},
		"play":       true,
	})
}

func (c *Client) Pause(ctx context.Context) error {
	return c.put(ctx, "/me/player/pause", nil)
}

func (c *Client) Next(ctx context.Context) error {
	return c.post(ctx, "/me/player/next", nil)
}

func (c *Client) Previous(ctx context.Context) error {
	return c.post(ctx, "/me/player/previous", nil)
}

func (c *Client) SetVolume(ctx context.Context, pct int) error {
	return c.put(ctx, fmt.Sprintf("/me/player/volume?volume_percent=%d", pct), nil)
}

func (c *Client) Seek(ctx context.Context, positionMs int) error {
	return c.put(ctx, fmt.Sprintf("/me/player/seek?position_ms=%d", positionMs), nil)
}

func (c *Client) Me(ctx context.Context) (*User, error) {
	var u User
	if err := c.get(ctx, "/me", &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) UserPlaylists(ctx context.Context) ([]Playlist, error) {
	// Spotify caps /me/playlists at 50 per request, so page through with offset
	// until "next" is null to list users with more than 50 playlists.
	var playlists []Playlist
	for offset := 0; ; offset += 50 {
		var result struct {
			Items []Playlist `json:"items"`
			Next  string     `json:"next"`
		}
		if err := c.get(ctx, fmt.Sprintf("/me/playlists?limit=50&offset=%d", offset), &result); err != nil {
			return nil, err
		}
		playlists = append(playlists, result.Items...)
		if result.Next == "" {
			break
		}
	}
	return playlists, nil
}

// TracksPageSize is the number of playlist items requested per page; it's also
// the Spotify-imposed maximum for the /items endpoint. Callers advance the
// offset by this amount to page through, regardless of how many items a page
// yields after filtering (podcast episodes / unavailable tracks are dropped).
const TracksPageSize = 100

// PlaylistTracksPage fetches one page of a playlist's tracks starting at offset.
// hasMore reports whether further pages exist, so callers can load the first
// page for an instant UI and fetch the rest in the background.
func (c *Client) PlaylistTracksPage(ctx context.Context, playlistID string, offset int) (tracks []Track, hasMore bool, err error) {
	// The /items endpoint nests each track under "item" (it can also hold
	// podcast episodes), unlike the legacy /tracks endpoint which used "track".
	var result struct {
		Items []struct {
			Track Track `json:"item"`
		} `json:"items"`
		Next string `json:"next"`
	}
	path := fmt.Sprintf("/playlists/%s/items?limit=%d&offset=%d", playlistID, TracksPageSize, offset)
	if err := c.get(ctx, path, &result); err != nil {
		return nil, false, err
	}
	tracks = make([]Track, 0, len(result.Items))
	for _, item := range result.Items {
		if item.Track.ID != "" {
			tracks = append(tracks, item.Track)
		}
	}
	return tracks, result.Next != "", nil
}

func (c *Client) Search(ctx context.Context, query string) ([]Track, []Playlist, error) {
	q := url.QueryEscape(query)

	var trackResult struct {
		Tracks struct {
			Items []Track `json:"items"`
		} `json:"tracks"`
	}
	if err := c.get(ctx, "/search?q="+q+"&type=track", &trackResult); err != nil {
		return nil, nil, err
	}

	// Playlist search may be restricted for new apps — fail silently.
	var plResult struct {
		Playlists struct {
			Items []Playlist `json:"items"`
		} `json:"playlists"`
	}
	_ = c.get(ctx, "/search?q="+q+"&type=playlist", &plResult)

	return trackResult.Tracks.Items, plResult.Playlists.Items, nil
}

func (c *Client) FetchImage(imageURL string) ([]byte, error) {
	resp, err := c.http.Get(imageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
