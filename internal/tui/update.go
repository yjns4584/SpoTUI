package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yesid/spotui/internal/pixelart"
	"github.com/yesid/spotui/internal/spotify"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case msgTick:
		cmds := []tea.Cmd{fetchPlayback(m.client), tickCmd()}
		if m.localPlayback() && m.localDeviceID == "" {
			cmds = append(cmds, fetchDevices(m.client))
		}
		return m, tea.Batch(cmds...)

	case msgDevices:
		if m.localPlayback() {
			for _, dev := range msg {
				if dev.Name == m.localDeviceName {
					m.localDeviceID = dev.ID
					break
				}
			}
		}
		return m, nil

	case msgPlaybackState:
		pb := spotify.PlaybackState(msg)
		m.playback = &pb
		m.err = nil
		m.playbackErr = "" // clear stale errors when fresh state arrives
		if pb.Item.Album.ID != "" {
			if _, ok := m.covers[pb.Item.Album.ID]; !ok {
				return m, fetchCover(m.client, pb.Item.Album.ID, pb.Item.Album.Images, m.coverSize)
			}
		}
		return m, nil

	case msgUser:
		m.userID = msg.ID
		return m, nil

	case msgPlaylists:
		m.playlists = []spotify.Playlist(msg)
		m.loading = false
		if len(m.playlists) > 0 {
			return m, fetchCoversForPlaylists(m.client, m.playlists[:min(5, len(m.playlists))], m.coverSize)
		}
		return m, nil

	case msgTracks:
		m.tracks = []spotify.Track(msg)
		m.trackCursor = 0
		m.loading = false
		m.err = nil
		m.activePane = PaneTrackList
		return m, nil

	case msgCoverRendered:
		m.covers[msg.key] = msg.rendered
		return m, nil

	case msgSearchResults:
		m.searching = false
		m.searchMode = false
		m.searchResults = true
		m.err = nil
		m.searchTracks = msg.tracks
		m.searchPlaylists = msg.playlists
		m.tracks = msg.tracks
		m.trackCursor = 0
		m.currentContext = "" // search results have no playlist context
		m.activePane = PaneTrackList
		return m, nil

	case msgPlaybackError:
		m.playbackErr = msg.err.Error()
		return m, nil

	case msgError:
		m.err = msg.err
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searchMode {
		return m.handleSearchKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc":
		if m.activeScreen != ScreenMain {
			m.activeScreen = ScreenMain
		}

	case " ":
		if m.playback != nil {
			deviceID := m.playback.Device.ID
			if deviceID == "" {
				deviceID = m.localDeviceID
			}
			return m, togglePlayback(m.client, deviceID, m.playback.IsPlaying)
		}

	case "d":
		if m.localDeviceID != "" {
			return m, transferPlayback(m.client, m.localDeviceID)
		}

	case "+", "=":
		if m.playback != nil {
			newVol := min(m.playback.Device.VolumePercent+5, 100)
			return m, setVolume(m.client, newVol)
		}

	case "-":
		if m.playback != nil {
			newVol := max(m.playback.Device.VolumePercent-5, 0)
			return m, setVolume(m.client, newVol)
		}

	case "tab":
		if m.activeScreen == ScreenMain {
			m.activePane = (m.activePane + 1) % 3
		}

	case "shift+tab":
		if m.activeScreen == ScreenMain {
			m.activePane = (m.activePane + 2) % 3
		}

	case "1":
		m.activeScreen = ScreenMain
	case "2":
		if m.activeScreen == ScreenNowPlaying {
			m.activeScreen = ScreenMain
		} else {
			m.activeScreen = ScreenNowPlaying
		}
	case "3":
		m.activeScreen = ScreenSearch
		m.searchMode = true

	case "/":
		m.searchMode = true
		m.activeScreen = ScreenSearch

	case "up", "k":
		m = m.moveCursorUp()
	case "down", "j":
		m = m.moveCursorDown()

	case "enter":
		return m.handleEnter()

	case "n":
		_ = m.client.Next(context.Background())
		return m, fetchPlayback(m.client)
	case "p":
		_ = m.client.Previous(context.Background())
		return m, fetchPlayback(m.client)
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.searching = false
		if m.searchResults {
			m.searchResults = false
			m.searchTracks = nil
			m.searchPlaylists = nil
			m.tracks = nil
			m.trackCursor = 0
			m.activePane = PaneLibrary
		}
		m.searchQuery = ""
	case "enter":
		if m.searchQuery != "" {
			m.searching = true
			m.err = nil
			m.activePane = PaneTrackList
			return m, search(m.client, m.searchQuery)
		}
		m.searchMode = false
	case "backspace":
		runes := []rune(m.searchQuery)
		if len(runes) > 0 {
			m.searchQuery = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			m.searchQuery += msg.String()
		}
	}
	return m, nil
}

func (m Model) moveCursorUp() Model {
	switch m.activePane {
	case PaneLibrary:
		if m.playlistCursor > 0 {
			m.playlistCursor--
		}
	case PaneTrackList:
		if m.trackCursor > 0 {
			m.trackCursor--
		}
	}
	return m
}

func (m Model) moveCursorDown() Model {
	switch m.activePane {
	case PaneLibrary:
		if m.playlistCursor < len(m.playlists)-1 {
			m.playlistCursor++
		}
	case PaneTrackList:
		if m.trackCursor < len(m.tracks)-1 {
			m.trackCursor++
		}
	}
	return m
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.activePane {
	case PaneLibrary:
		if m.playlistCursor < len(m.playlists) {
			pl := m.playlists[m.playlistCursor]
			if !m.canReadTracks(pl) {
				m.err = fmt.Errorf("followed playlist — only your own playlists are accessible. Use [/] search instead")
				m.tracks = nil
				m.activePane = PaneTrackList
				return m, nil
			}
			m.loading = true
			m.err = nil
			m.currentContext = "spotify:playlist:" + pl.ID
			return m, fetchPlaylistTracks(m.client, pl.ID)
		}
	case PaneTrackList:
		if m.trackCursor < len(m.tracks) {
			track := m.tracks[m.trackCursor]
			return m, playTrack(m.client, m.localDeviceID, m.currentContext, "spotify:track:"+track.ID)
		}
	}
	return m, nil
}

// ── Cover fetching ────────────────────────────────────────────────────────────

// coverHeight returns the char height for a square-looking cover at the given
// char width. The ▀ block packs 2 pixels per cell and terminal cells are ~2:1
// (tall:wide), so a square image needs height = width / 2 character rows.
func coverHeight(w int) int {
	h := w / 2
	if h < 1 {
		return 1
	}
	return h
}

func fetchCover(c *spotify.Client, id string, images []spotify.Image, size int) tea.Cmd {
	url := bestImage(images, 300)
	if url == "" {
		return nil
	}
	return func() tea.Msg {
		data, err := c.FetchImage(url)
		if err != nil {
			return nil
		}
		rendered, err := pixelart.RenderPixelated(data, size, coverHeight(size), 8)
		if err != nil {
			return nil
		}
		return msgCoverRendered{key: id, rendered: rendered}
	}
}

func fetchCoversForPlaylists(c *spotify.Client, pls []spotify.Playlist, size int) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(pls))
	for _, pl := range pls {
		pl := pl
		cmds = append(cmds, fetchCover(c, pl.ID, pl.Images, size))
	}
	return tea.Batch(cmds...)
}

func bestImage(images []spotify.Image, targetSize int) string {
	if len(images) == 0 {
		return ""
	}
	best := images[0]
	for _, img := range images {
		if abs(img.Width-targetSize) < abs(best.Width-targetSize) {
			best = img
		}
	}
	return best.URL
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
