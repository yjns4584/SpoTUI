package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/yesid/spotui/internal/tui/styles"
)

// contentW returns the usable text width inside a panel given its total width.
// Total panel = content + padding(1+1) + border(1+1) = content + 4.
func contentW(panelW int) int {
	return panelW - 4
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}
	switch m.activeScreen {
	case ScreenNowPlaying:
		return m.viewNowPlaying()
	default:
		return m.viewMain()
	}
}

// ── Main 3-pane layout ────────────────────────────────────────────────────────

func (m Model) viewMain() string {
	totalH := m.height - 2 // top bar + status bar

	libraryW := 32
	playerW := m.coverSize + 4 // content = coverSize, +4 for padding+border
	trackW := m.width - libraryW - playerW
	if trackW < 20 {
		trackW = 20
	}

	library := m.viewLibrary(libraryW, totalH)
	tracks := m.viewTrackList(trackW, totalH)
	player := m.viewPlayer(playerW, totalH)

	body := lipgloss.JoinHorizontal(lipgloss.Top, library, tracks, player)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewTopBar(),
		body,
		m.viewStatusBar(),
	)
}

func (m Model) viewTopBar() string {
	t := m.theme
	logo := lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render("♫ SpoTUI")

	right := ""
	if m.playback != nil && m.playback.IsPlaying {
		right = lipgloss.NewStyle().Foreground(t.Green).Render("● live")
	} else if m.playback != nil {
		right = lipgloss.NewStyle().Foreground(t.Dim).Render("◌ paused")
	}

	pad := max(0, m.width-lipgloss.Width(logo)-lipgloss.Width(right)-4)
	return lipgloss.NewStyle().
		Background(t.BgAlt).
		Foreground(t.Text).
		Width(m.width).
		Render(logo + strings.Repeat(" ", pad+2) + right)
}

func (m Model) viewStatusBar() string {
	t := m.theme

	var keys string
	if m.searchMode {
		keys = lipgloss.NewStyle().Foreground(t.Dim).Render("[esc] cancel  [enter] search  [backspace] delete")
	} else if m.searchResults {
		keys = lipgloss.NewStyle().Foreground(t.Dim).Render("[esc] clear  [enter] play  [tab] pane")
	} else {
		pairs := []struct{ key, label string }{
			{"space", "play/pause"}, {"n/p", "skip"}, {"+/-", "volume"},
		}
		if m.localPlayback() {
			pairs = append(pairs, struct{ key, label string }{"d", "play here"})
		}
		pairs = append(pairs, []struct{ key, label string }{
			{"/", "search"}, {"2", "now playing"}, {"q", "quit"},
		}...)
		var parts []string
		for _, k := range pairs {
			parts = append(parts,
				lipgloss.NewStyle().Foreground(t.Muted).Render("["+k.key+"]")+
					lipgloss.NewStyle().Foreground(t.Muted).Render(" "+k.label),
			)
		}
		keys = strings.Join(parts, "  ")
	}

	// Local device indicator on the right side.
	local := ""
	if m.localPlayback() {
		active := m.playback != nil && m.playback.Device.ID == m.localDeviceID && m.localDeviceID != ""
		switch {
		case active:
			local = lipgloss.NewStyle().Foreground(t.Green).Render("◉ " + m.localDeviceName)
		case m.localDeviceID != "":
			local = lipgloss.NewStyle().Foreground(t.Muted).Render("○ " + m.localDeviceName + " ready")
		default:
			local = lipgloss.NewStyle().Foreground(t.Dim).Render("○ starting " + m.localDeviceName + "...")
		}
	}

	pad := max(0, m.width-lipgloss.Width(keys)-lipgloss.Width(local)-4)
	return lipgloss.NewStyle().
		Background(t.BgAlt).
		Width(m.width).
		Render("  " + keys + strings.Repeat(" ", pad) + local + "  ")
}

// ── Library panel ─────────────────────────────────────────────────────────────

func (m Model) viewLibrary(w, h int) string {
	t := m.theme
	focused := m.activePane == PaneLibrary
	cw := contentW(w)
	textW := cw - 2

	var sb strings.Builder
	title := lipgloss.NewStyle().Foreground(t.Dim).Render("─ library ─")

	// When search has playlist results, show them instead of the library
	if m.searchResults && len(m.searchPlaylists) > 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("  playlists") + "\n")
		for i, pl := range m.searchPlaylists {
			name := truncate(pl.Name, textW)
			line := "  " + name
			var style lipgloss.Style
			if i == m.playlistCursor && focused {
				style = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
			} else {
				style = lipgloss.NewStyle().Foreground(t.Text)
			}
			sb.WriteString(lipgloss.NewStyle().MaxWidth(cw).Render(style.Render(line)) + "\n")
		}
		title = lipgloss.NewStyle().Foreground(t.Accent2).Render("─ results ─")
		return renderPanel(t, focused, w, h, title, sb.String())
	}

	for i, pl := range m.playlists {
		owned := m.canReadTracks(pl)
		prefix := "  "
		if !owned {
			prefix = "↳ "
		}
		name := truncate(pl.Name, textW)
		line := prefix + name

		var style lipgloss.Style
		if i == m.playlistCursor {
			if focused {
				style = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
			} else {
				style = lipgloss.NewStyle().Foreground(t.Text).Bold(true)
			}
		} else if owned {
			style = lipgloss.NewStyle().Foreground(t.Text)
		} else {
			style = lipgloss.NewStyle().Foreground(t.Muted)
		}
		sb.WriteString(lipgloss.NewStyle().MaxWidth(cw).Render(style.Render(line)) + "\n")
	}

	return renderPanel(t, focused, w, h, title, sb.String())
}

// ── Track list panel ──────────────────────────────────────────────────────────

func (m Model) viewTrackList(w, h int) string {
	t := m.theme
	focused := m.activePane == PaneTrackList
	cw := contentW(w)

	var sb strings.Builder

	// Search input bar
	if m.searchMode || m.searchResults {
		cursor := ""
		if m.searchMode {
			cursor = lipgloss.NewStyle().Foreground(t.Accent).Render("█")
		}
		inputBox := lipgloss.NewStyle().
			Foreground(t.Accent).Render("/") + " " +
			lipgloss.NewStyle().Foreground(t.Text).Render(m.searchQuery) +
			cursor
		border := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(t.Border).
			Width(cw).
			Render(inputBox)
		sb.WriteString(border + "\n")
	}

	// State messages
	if m.err != nil {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).
			Render("  "+m.err.Error()) + "\n")
	} else if m.searching {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("  searching...") + "\n")
	} else if m.loading {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("  loading...") + "\n")
	} else if m.searchResults && len(m.tracks) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("  no tracks found") + "\n")
	} else if !m.searchMode && !m.searchResults && len(m.tracks) == 0 && len(m.playlists) > 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("  empty playlist or local files only") + "\n")
	} else if len(m.tracks) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("  select a playlist · press [/] to search") + "\n")
	}

	numW := 3
	durW := 5
	nameW := cw - numW - durW - 1 // 1 char minimum gap between name and duration

	// Sliding window so the cursor is always visible. renderPanel clips
	// content to h-6 lines, so we keep visible tracks within that budget.
	overhead := 6
	if m.searchMode || m.searchResults {
		overhead += 2
	}
	visibleTracks := (h - overhead) / 2
	if visibleTracks < 1 {
		visibleTracks = 1
	}
	start := 0
	if len(m.tracks) > visibleTracks && m.trackCursor >= visibleTracks {
		start = m.trackCursor - visibleTracks + 1
	}
	end := start + visibleTracks
	if end > len(m.tracks) {
		end = len(m.tracks)
	}

	dimS := lipgloss.NewStyle().Foreground(t.Dim)
	for i := start; i < end; i++ {
		tr := m.tracks[i]
		name := truncate(tr.Name, nameW)
		artist := truncate(tr.ArtistsString(), cw-numW)
		dur := fmtDuration(tr.Duration)
		num := fmt.Sprintf("%2d ", i+1)

		var numS, nameS lipgloss.Style
		if i == m.trackCursor {
			if focused {
				numS = lipgloss.NewStyle().Foreground(t.Accent)
				nameS = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
			} else {
				numS = lipgloss.NewStyle().Foreground(t.Text)
				nameS = lipgloss.NewStyle().Foreground(t.Text).Bold(true)
			}
		} else {
			numS = dimS
			nameS = lipgloss.NewStyle().Foreground(t.Text)
		}

		// Pad so total visible width = cw exactly: num(3) + name + pad + dur
		pad := cw - numW - lipgloss.Width(name) - lipgloss.Width(dur)
		if pad < 1 {
			pad = 1
		}
		line1 := numS.Render(num) + nameS.Render(name) +
			strings.Repeat(" ", pad) + dimS.Render(dur)
		line2 := strings.Repeat(" ", numW) + dimS.Render(artist)

		sb.WriteString(line1 + "\n")
		sb.WriteString(line2 + "\n")
	}

	var titleStr string
	switch {
	case m.searchResults:
		q := truncate(m.searchQuery, 20)
		titleStr = lipgloss.NewStyle().Foreground(t.Accent2).Render("─ results: "+q+" ─")
	case m.searching:
		titleStr = lipgloss.NewStyle().Foreground(t.Accent).Render("─ searching ─")
	default:
		titleStr = lipgloss.NewStyle().Foreground(t.Dim).Render("─ tracks ─")
	}
	return renderPanel(t, focused, w, h, titleStr, sb.String())
}

// ── Player panel ──────────────────────────────────────────────────────────────

func (m Model) viewPlayer(w, h int) string {
	t := m.theme
	focused := m.activePane == PanePlayer
	cw := contentW(w)

	var sb strings.Builder

	if m.playback == nil || m.playback.Item.ID == "" {
		msg := "♫\n\nno active playback"
		if m.playbackErr != "" {
			msg = "♫\n\n" + m.playbackErr
		}
		body := lipgloss.NewStyle().
			Width(cw).Height(h-4).
			Foreground(t.Dim).
			Align(lipgloss.Center, lipgloss.Center).
			Render(msg)
		sb.WriteString(body)
		return renderPanel(t, focused, w, h, playerTitle(t), sb.String())
	}

	pb := m.playback

	center := func(s string) string {
		return lipgloss.NewStyle().Width(cw).Align(lipgloss.Center).Render(s)
	}

	// Cover art
	if cover, ok := m.covers[pb.Item.Album.ID]; ok {
		sb.WriteString(cover)
	} else {
		sb.WriteString(placeholderCover(m.coverSize) + "\n")
	}
	sb.WriteString("\n")

	// Track title / artist / album
	sb.WriteString(center(lipgloss.NewStyle().Foreground(t.Accent).Bold(true).
		Render(truncate(pb.Item.Name, cw))) + "\n")
	sb.WriteString(center(lipgloss.NewStyle().Foreground(t.Text).
		Render(truncate(pb.Item.ArtistsString(), cw))) + "\n")
	sb.WriteString(center(lipgloss.NewStyle().Foreground(t.Muted).
		Render(truncate(pb.Item.Album.Name, cw))) + "\n\n")

	// Play state
	if pb.IsPlaying {
		sb.WriteString(center(lipgloss.NewStyle().Foreground(t.Green).Bold(true).Render("▶  playing")) + "\n\n")
	} else {
		sb.WriteString(center(lipgloss.NewStyle().Foreground(t.Dim).Render("⏸  paused")) + "\n\n")
	}

	// Progress bar with elapsed (left) and total (right)
	sb.WriteString(progressBar(pb.ProgressMs, pb.Item.Duration, cw, t) + "\n")
	elapsed := fmtDuration(pb.ProgressMs)
	total := fmtDuration(pb.Item.Duration)
	gap := max(0, cw-lipgloss.Width(elapsed)-lipgloss.Width(total))
	timeStyle := lipgloss.NewStyle().Foreground(t.Dim)
	sb.WriteString(timeStyle.Render(elapsed) + strings.Repeat(" ", gap) +
		timeStyle.Render(total) + "\n")

	// Device + volume
	if pb.Device.Name != "" {
		sb.WriteString("\n")
		sb.WriteString(center(lipgloss.NewStyle().Foreground(t.Muted).
			Render(truncate("♪ "+pb.Device.Name, cw))) + "\n")
		sb.WriteString(volumeBar(pb.Device.VolumePercent, cw, t) + "\n")
	}

	// Playback error
	if m.playbackErr != "" {
		sb.WriteString("\n")
		for _, line := range wrapText(m.playbackErr, cw) {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Render(line) + "\n")
		}
	}

	return renderPanel(t, focused, w, h, playerTitle(t), sb.String())
}

func playerTitle(t styles.Theme) string {
	return lipgloss.NewStyle().Foreground(t.Accent).Render("─ player ─")
}

// ── Now Playing fullscreen ────────────────────────────────────────────────────

func (m Model) viewNowPlaying() string {
	t := m.theme

	statusBar := lipgloss.NewStyle().
		Background(t.BgAlt).
		Foreground(t.Dim).
		Width(m.width).
		Render("  [esc] back  [space] play/pause  [n] next  [p] prev  [q] quit")

	if m.playback == nil || m.playback.Item.ID == "" {
		body := lipgloss.NewStyle().
			Width(m.width).Height(m.height-1).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(t.Dim).
			Render("no active playback")
		return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
	}

	pb := m.playback
	coverSize := min(m.coverSize*2, 64)

	var sb strings.Builder
	if cover, ok := m.covers[pb.Item.Album.ID]; ok {
		sb.WriteString(cover)
	} else {
		sb.WriteString(placeholderCover(coverSize) + "\n")
	}
	sb.WriteString("\n")

	if pb.IsPlaying {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Green).Bold(true).Render("▶ playing") + "\n\n")
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("⏸ paused") + "\n\n")
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(t.Accent).Bold(true).
		Render(pb.Item.Name) + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Text).
		Render(pb.Item.ArtistsString()) + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Muted).
		Render(pb.Item.Album.Name) + "\n\n")

	sb.WriteString(progressBar(pb.ProgressMs, pb.Item.Duration, coverSize, t) + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Dim).
		Render(fmtDuration(pb.ProgressMs)+" / "+fmtDuration(pb.Item.Duration)) + "\n")

	if pb.Device.Name != "" {
		sb.WriteString("\n" + lipgloss.NewStyle().Foreground(t.Muted).
			Render(pb.Device.Name+" · "+fmt.Sprintf("%d%%", pb.Device.VolumePercent)) + "\n")
	}

	body := lipgloss.NewStyle().
		Width(m.width).Height(m.height-1).
		Align(lipgloss.Center, lipgloss.Center).
		Render(sb.String())

	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func renderPanel(t styles.Theme, focused bool, w, h int, title, content string) string {
	// Lipgloss treats Width() as inclusive of padding (wrap point = width - padding),
	// so to fit content of cw = w-4 chars we pass Width(w-2): wrap point becomes
	// (w-2) - 2 = w-4 = cw. Total block = (w-2) + 2 border = w.
	style := styles.PanelStyle(t, focused).Width(w - 2).Height(h - 2)
	lines := strings.Split(content, "\n")
	maxLines := h - 6
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	clipped := strings.Join(lines, "\n")
	return style.Render(title + "\n" + clipped)
}

func progressBar(progressMs, totalMs, width int, t styles.Theme) string {
	if totalMs == 0 || width <= 0 {
		return lipgloss.NewStyle().Foreground(t.Muted).Render(strings.Repeat("░", width))
	}
	filled := int(float64(progressMs) / float64(totalMs) * float64(width))
	filled = min(filled, width)
	return lipgloss.NewStyle().Foreground(t.Accent).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(t.Muted).Render(strings.Repeat("░", width-filled))
}

func volumeBar(pct, width int, t styles.Theme) string {
	if width <= 0 {
		return ""
	}
	barW := width - 5 // reserve 5 chars for "100% " suffix
	if barW < 1 {
		barW = 1
	}
	filled := int(float64(pct) / 100.0 * float64(barW))
	filled = min(filled, barW)
	bar := lipgloss.NewStyle().Foreground(t.Accent2).Render(strings.Repeat("▪", filled)) +
		lipgloss.NewStyle().Foreground(t.Muted).Render(strings.Repeat("▪", barW-filled))
	label := lipgloss.NewStyle().Foreground(t.Dim).Render(fmt.Sprintf(" %3d%%", pct))
	return bar + label
}

// placeholderCover renders a subtle solid block with a musical note in the
// middle. Used as the cover stand-in while the real image is loading.
func placeholderCover(size int) string {
	rows := coverHeight(size)
	cr, cc := rows/2, size/2
	var sb strings.Builder
	for r := 0; r < rows; r++ {
		for c := 0; c < size; c++ {
			if r == cr && c == cc {
				sb.WriteString("\x1b[38;2;127;132;156;48;2;49;50;68m♫")
			} else {
				sb.WriteString("\x1b[38;2;49;50;68;48;2;49;50;68m▀")
			}
		}
		sb.WriteString("\x1b[0m\n")
	}
	return sb.String()
}

// truncate clips s to maxWidth terminal display columns, appending "…".
// Uses lipgloss.Width() so emoji and CJK characters (width=2) are counted correctly.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	for i := len(runes) - 1; i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return "…"
}

func fmtDuration(ms int) string {
	s := ms / 1000
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// wrapText splits s into lines of at most maxWidth display columns.
func wrapText(s string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{s}
	}
	var lines []string
	words := strings.Fields(s)
	cur := ""
	for _, w := range words {
		candidate := w
		if cur != "" {
			candidate = cur + " " + w
		}
		if lipgloss.Width(candidate) <= maxWidth {
			cur = candidate
		} else {
			if cur != "" {
				lines = append(lines, cur)
			}
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

