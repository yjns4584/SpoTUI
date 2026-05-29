package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yesid/spotui/internal/spotify"
	"github.com/yesid/spotui/internal/tui/styles"
)

func debugLog(msg string) {
	dir, _ := os.UserConfigDir()
	f, err := os.OpenFile(filepath.Join(dir, "spotui", "debug.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("15:04:05"), msg)
}

type Pane int

const (
	PaneLibrary Pane = iota
	PaneTrackList
	PanePlayer
)

type Screen int

const (
	ScreenMain Screen = iota
	ScreenNowPlaying
	ScreenSearch
	ScreenQueue
)

// ── Messages ─────────────────────────────────────────────────────────────────

type msgPlaybackState spotify.PlaybackState
type msgUser spotify.User
type msgPlaylists []spotify.Playlist
type msgTracksPage struct {
	playlistID string
	offset     int
	tracks     []spotify.Track
	hasMore    bool
}
type msgCoverRendered struct {
	key      string // playlist/album ID
	rendered string // ANSI art string
}
type msgTick time.Time
type msgError struct{ err error }        // error shown to the user (data fetches)
type msgPlaybackError struct{ err error } // silent: playback errors don't block the UI
type msgSearchResults struct {
	tracks    []spotify.Track
	playlists []spotify.Playlist
}
type msgDevices []spotify.Device

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	client *spotify.Client
	theme  styles.Theme
	width  int
	height int

	// navigation
	activePane   Pane
	activeScreen Screen

	// library panel
	playlists      []spotify.Playlist
	playlistCursor int

	// track list panel
	tracks         []spotify.Track
	trackCursor    int
	currentContext string // Spotify URI giving Play a queue (e.g. "spotify:playlist:..."); empty = no context

	// player
	playback  *spotify.PlaybackState
	covers    map[string]string // id -> rendered ANSI art
	coverSize int               // char width for covers

	// search
	searchMode      bool
	searchQuery     string
	searching       bool             // waiting for API response
	searchResults   bool             // currently showing search results
	searchTracks    []spotify.Track
	searchPlaylists []spotify.Playlist

	// user
	userID string

	// local playback (embedded spotifyd device)
	localDeviceName string // "SpoTUI" when spotifyd is running, "" otherwise
	localDeviceID   string // discovered Connect device ID for localDeviceName

	// state
	loading     bool
	loadingMore bool   // background playlist pages still streaming in
	err         error  // error from data operations (shown in track panel)
	playbackErr string // last playback error (shown in player panel, non-blocking)
}

// localPlayback reports whether the embedded spotifyd device is available.
func (m Model) localPlayback() bool {
	return m.localDeviceName != ""
}

// canReadTracks returns true if the playlist belongs to the current user
// or is collaborative — the only cases Spotify allows reading tracks for
// apps without Extended Access.
func (m Model) canReadTracks(pl spotify.Playlist) bool {
	if pl.Collaborative {
		return true
	}
	if m.userID == "" {
		return true // unknown yet, try anyway
	}
	return pl.Owner.ID == m.userID
}

func NewModel(client *spotify.Client, localDeviceName string) Model {
	return Model{
		client:          client,
		theme:           styles.Active,
		covers:          make(map[string]string),
		coverSize:       40,
		localDeviceName: localDeviceName,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchMe(m.client),
		fetchPlaylists(m.client),
		fetchPlayback(m.client),
		fetchDevices(m.client),
		tickCmd(),
	)
}

func fetchDevices(c *spotify.Client) tea.Cmd {
	return func() tea.Msg {
		devices, err := c.ListDevices(context.Background())
		if err != nil {
			return nil
		}
		return msgDevices(devices)
	}
}

func fetchMe(c *spotify.Client) tea.Cmd {
	return func() tea.Msg {
		u, err := c.Me(context.Background())
		if err != nil {
			return nil
		}
		return msgUser(*u)
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return msgTick(t)
	})
}

func fetchPlaylists(c *spotify.Client) tea.Cmd {
	return func() tea.Msg {
		pl, err := c.UserPlaylists(context.Background())
		if err != nil {
			return msgError{err}
		}
		return msgPlaylists(pl)
	}
}

func fetchPlayback(c *spotify.Client) tea.Cmd {
	return func() tea.Msg {
		pb, err := c.CurrentPlayback(context.Background())
		if err != nil {
			debugLog("playback error: " + err.Error())
			return msgPlaybackError{err} // silent — doesn't block UI
		}
		if pb == nil {
			return nil
		}
		return msgPlaybackState(*pb)
	}
}

// fetchPlaylistTracksPage loads a single page of a playlist's tracks. The first
// page (offset 0) renders immediately; the msgTracksPage handler then chains
// further pages so large playlists fill in without blocking the UI.
func fetchPlaylistTracksPage(c *spotify.Client, playlistID string, offset int) tea.Cmd {
	return func() tea.Msg {
		debugLog(fmt.Sprintf("fetching tracks for playlist %s @ offset %d", playlistID, offset))
		tracks, hasMore, err := c.PlaylistTracksPage(context.Background(), playlistID, offset)
		if err != nil {
			debugLog("tracks error: " + err.Error())
			return msgError{err}
		}
		debugLog(fmt.Sprintf("tracks page loaded: %d (more: %v)", len(tracks), hasMore))
		return msgTracksPage{playlistID: playlistID, offset: offset, tracks: tracks, hasMore: hasMore}
	}
}

func search(c *spotify.Client, query string) tea.Cmd {
	return func() tea.Msg {
		tracks, playlists, err := c.Search(context.Background(), query)
		if err != nil {
			return msgError{err}
		}
		return msgSearchResults{tracks: tracks, playlists: playlists}
	}
}

func playTrack(c *spotify.Client, deviceID, contextURI, trackURI string) tea.Cmd {
	return func() tea.Msg {
		if err := c.Play(context.Background(), deviceID, contextURI, trackURI); err != nil {
			return msgPlaybackError{err}
		}
		time.Sleep(300 * time.Millisecond)
		return fetchPlayback(c)()
	}
}

func togglePlayback(c *spotify.Client, deviceID string, isPlaying bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if isPlaying {
			err = c.Pause(context.Background())
		} else {
			err = c.Play(context.Background(), deviceID, "", "")
		}
		if err != nil {
			return msgPlaybackError{err}
		}
		time.Sleep(200 * time.Millisecond)
		return fetchPlayback(c)()
	}
}

func transferPlayback(c *spotify.Client, deviceID string) tea.Cmd {
	return func() tea.Msg {
		if err := c.TransferPlayback(context.Background(), deviceID); err != nil {
			return msgPlaybackError{err}
		}
		time.Sleep(500 * time.Millisecond)
		return fetchPlayback(c)()
	}
}

func setVolume(c *spotify.Client, pct int) tea.Cmd {
	return func() tea.Msg {
		if err := c.SetVolume(context.Background(), pct); err != nil {
			return msgPlaybackError{err}
		}
		time.Sleep(200 * time.Millisecond)
		return fetchPlayback(c)()
	}
}
