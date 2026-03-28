package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/purplefish32/riff/internal/types"
)

// --- View helpers ---

func (a App) searchVisible() bool {
	return a.mode == modeSearchInput || a.mode == modeSearchBrowse
}

func (a App) dlCheck() func(types.Track) bool {
	if a.dl == nil {
		return func(types.Track) bool { return false }
	}
	return a.dl.IsDownloaded
}

func (a App) renderTabBar() string {
	type tabEntry struct {
		label string
		tab   viewTab
	}
	plLabel := "Playlists"
	if a.playlists != nil {
		if names := a.playlists.List(); len(names) > 0 {
			plLabel = fmt.Sprintf("Playlists(%d)", len(names))
		}
	}
	recentLabel := "Recent"
	if a.recent != nil {
		if n := len(a.recent.List()); n > 0 {
			recentLabel = fmt.Sprintf("Recent(%d)", n)
		}
	}
	tabs := []tabEntry{
		{"Queue", tabQueue},
		{recentLabel, tabRecent},
		{plLabel, tabPlaylists},
	}

	dimmedAll := a.searchVisible() || a.mode == modeHelp || a.mode == modeFilter || a.mode == modeSavePlaylist || a.mode == modeRenamePlaylist || a.mode == modeAddToPlaylist || a.mode == modeConfirmDelete

	selCount := len(a.selected)
	var parts []string
	for _, t := range tabs {
		label := t.label
		if t.tab == tabQueue && len(a.tracklist) > 0 {
			if a.activePlaylist != "" {
				dirty := ""
				if a.playlistDirty {
					dirty = "*"
				}
				label = fmt.Sprintf("Queue [%s%s](%d)", a.activePlaylist, dirty, len(a.tracklist))
			} else {
				label = fmt.Sprintf("Queue(%d)", len(a.tracklist))
			}
		}
		if selCount > 0 && t.tab == a.activeTab {
			label += fmt.Sprintf(" [%d sel]", selCount)
		}
		if !dimmedAll && t.tab == a.activeTab {
			parts = append(parts, activeTabStyle.Render(" "+label+" "))
		} else {
			parts = append(parts, dimStyle.Render(" "+label+" "))
		}
	}
	return "  " + strings.Join(parts, dimStyle.Render("│"))
}

// isFilterMatch reports whether index i is in the filtered set.
// When filter is inactive (filterText == ""), all indices match.
func (a App) isFilterMatch(i int) bool {
	if a.filterText == "" {
		return true
	}
	for _, fi := range a.filteredIndices {
		if fi == i {
			return true
		}
	}
	return false
}

func (a App) renderQueueView() string {
	if len(a.tracklist) == 0 {
		return dimStyle.Render("  Queue is empty. Press 'a' on a track to add it.")
	}

	visibleRows := a.visibleRows()

	tc := computeTrackCols(a.width)
	s := ""
	if a.mode == modeFilter {
		s += "  " + a.filterInput.View()
		if a.filterText != "" && len(a.filteredIndices) == 0 {
			s += "  " + dimStyle.Render("No matches")
		}
		s += "\n"
	}
	// Queue header (respects showLineNumbers)
	qh := "   "
	if a.showLineNumbers {
		qh += colRight("#", colNum, headerStyle)
	}
	qh += col("Artist", tc.artist, headerStyle) +
		col("Title", tc.title, headerStyle)
	if tc.showAlbum {
		qh += col("Album", tc.album, headerStyle)
	}
	if tc.showYear {
		qh += colRight("Year", colYear, headerStyle)
	}
	qh += colRight("Time", colDuration, headerStyle)
	s += qh + "\n"

	end := a.queueScrollOffset + visibleRows
	if end > len(a.tracklist) {
		end = len(a.tracklist)
	}

	if a.queueScrollOffset > 0 {
		s += dimStyle.Render(fmt.Sprintf("  ^ %d more above", a.queueScrollOffset)) + "\n"
	}

	for displayIdx, i := 0, a.queueScrollOffset; i < end; i, displayIdx = i+1, displayIdx+1 {
		t := a.tracklist[i]
		isPlaying := i == a.trackPos
		isCursor := i == a.queueCursor
		played := a.trackPos >= 0 && i < a.trackPos
		isMatch := a.isFilterMatch(i)

		duration := fmt.Sprintf("%d:%02d", t.Duration/60, t.Duration%60)
		num := fmt.Sprintf("%d", i+1)
		isSelected := a.selected[i]
		icons := statusIcons(a.likes.IsLiked(t.ID), a.dl != nil && a.dl.IsDownloaded(t))

		var numSt, artSt, albSt, titSt, durSt lipgloss.Style
		var marker string

		if !isMatch {
			marker = " "
			numSt, artSt, albSt, titSt, durSt = dimStyle, dimStyle, dimStyle, dimStyle, dimStyle
		} else {
			switch {
			case isCursor && isPlaying:
				marker = titleStyle.Render("▸")
				numSt, artSt, albSt, titSt, durSt = playingSelectedStyle, playingSelectedStyle, playingSelectedStyle, playingSelectedStyle, playingSelectedStyle
			case isPlaying:
				marker = playingStyle.Render("♫")
				numSt, artSt, albSt, titSt, durSt = playingStyle, playingStyle, playingStyle, playingStyle, playingStyle
			case isCursor:
				marker = selectionStripe.Render("▸")
				if noColor {
					numSt, artSt, albSt, titSt, durSt = selectedStyle, selectedStyle, selectedStyle, selectedStyle, selectedStyle
				} else {
					numSt, artSt, albSt, titSt, durSt = dimStyle.Bold(true), artistStyle.Bold(true), dimStyle.Bold(true), normalStyle.Bold(true), dimStyle.Bold(true)
				}
			case played:
				marker = " "
				numSt, artSt, albSt, titSt, durSt = dimStyle, dimStyle, dimStyle, dimStyle, dimStyle
			default:
				marker = " "
				numSt, artSt, albSt, titSt, durSt = dimStyle, artistStyle, dimStyle, normalStyle, dimStyle
			}
		}
		if isSelected {
			marker = titleStyle.Render("●")
		}

		// Prepend album track number to title when available
		titleText := t.Title
		if t.TrackNumber > 0 {
			titleText = fmt.Sprintf("%02d. %s", t.TrackNumber, t.Title)
		}

		// Play count suffix
		playCountSuffix := ""
		if a.showPlayCounts && a.playCounts != nil {
			if cnt := a.playCounts.Get(t.ID); cnt > 0 {
				playCountSuffix = dimStyle.Render(fmt.Sprintf(" ×%d", cnt))
			}
		}

		var row string
		if tc.artist == 0 {
			// Ultra-narrow: title only
			row = marker + icons + col(titleText, tc.title, titSt) + playCountSuffix
		} else {
			row = marker + icons
			if a.showLineNumbers {
				row += colRight(num, colNum, numSt)
			}
			row +=	col(t.Artist.Name, tc.artist, artSt) +
				col(titleText, tc.title, titSt)
			if tc.showAlbum {
				row += col(t.Album.Title, tc.album, albSt)
			}
			if tc.showYear {
				row += colRight(trackYear(t), colYear, durSt)
			}
			row += colRight(duration, colDuration, durSt) + playCountSuffix
		}
		s += row + "\n"
	}

	if end < len(a.tracklist) {
		s += dimStyle.Render(fmt.Sprintf("  v %d more below", len(a.tracklist)-end)) + "\n"
	}

	return s
}


func (a App) renderRecentView() string {
	if a.recent == nil {
		return dimStyle.Render("  Recent unavailable.")
	}
	tracks := a.recent.List()
	if len(tracks) == 0 {
		return dimStyle.Render("  No recently played tracks yet.")
	}

	visibleRows := a.visibleRows()

	tc := computeTrackCols(a.width)

	// Header
	h := "   " +
		col("Artist", tc.artist, headerStyle) +
		col("Title", tc.title, headerStyle)
	if tc.showAlbum {
		h += col("Album", tc.album, headerStyle)
	}
	if tc.showYear {
		h += colRight("Year", colYear, headerStyle)
	}
	h += colRight("Time", colDuration, headerStyle)
	s := h + "\n"

	end := a.recentScrollOffset + visibleRows
	if end > len(tracks) {
		end = len(tracks)
	}

	if a.recentScrollOffset > 0 {
		s += dimStyle.Render(fmt.Sprintf("  ^ %d more above", a.recentScrollOffset)) + "\n"
	}

	isDownloaded := a.dlCheck()
	for i := a.recentScrollOffset; i < end; i++ {
		t := tracks[i]
		selected := i == a.recentCursor
		liked := a.likes.IsLiked(t.ID)
		downloaded := isDownloaded(t)
		s += trackRow(i, t, selected, liked, downloaded, tc) + "\n"
	}

	if end < len(tracks) {
		s += dimStyle.Render(fmt.Sprintf("  v %d more below", len(tracks)-end)) + "\n"
	}

	return s
}

func (a App) renderPlaylistsView() string {
	names := a.playlistNames
	if len(names) == 0 && a.playlists != nil {
		names = a.playlists.List()
	}
	if len(names) == 0 {
		return dimStyle.Render("  No saved playlists. Use :save <name> to create one.")
	}

	s := "   " + headerStyle.Render("Playlist") + "\n"
	for i, name := range names {
		if i == a.playlistCursor {
			s += selectionStripe.Render("▸") + "  " + normalStyle.Bold(true).Render(name) + "\n"
		} else {
			s += "   " + normalStyle.Render(name) + "\n"
		}
	}
	return s
}

func (a App) View() tea.View {
	// --- Header ---
	var header string
	if a.height < 20 {
		onlineSuffix := ""
		if !a.online {
			onlineSuffix = "  " + errorStyle.Render("OFFLINE")
		}
		header = "  " + titleStyle.Render("riff") + onlineSuffix
	} else {
		logo := "  " + titleStyle.Render("╦═╗╦╔═╗╔═╗") + "\n" +
			"  " + titleStyle.Render("╠╦╝║╠╣ ╠╣ ") + "\n" +
			"  " + titleStyle.Render("╩╚═╩╚  ╚  ")
		if !a.online {
			logo += "  " + errorStyle.Render("OFFLINE")
		}
		header = logo
	}

	// --- Tab bar ---
	tabBar := a.renderTabBar()

	// --- Now playing / loading bar ---
	spinner := spinnerFrames[a.spinnerIdx]
	var np string
	if a.loading {
		np = nowPlayingStyle.Render(dimStyle.Render("  " + spinner + " Loading stream..."))
	} else {
		np = a.nowPlaying.View(a.width)
	}

	// --- Error view ---
	errView := ""
	if a.err != nil {
		errView = errorStyle.Render(fmt.Sprintf("  Error: %s", a.err))
	}

	// --- Status line ---
	statusLine := ""
	if a.statusMsg != "" {
		var statusStyle lipgloss.Style
		if a.statusTicks <= 1 {
			statusStyle = dimStyle
		} else {
			statusStyle = titleStyle
		}
		statusLine = "  " + statusStyle.Render(a.statusMsg)
	}

	// --- Download status ---
	dlStatus := ""
	if a.dl != nil {
		st := a.dl.Status()
		if st.Active > 0 || st.Queued > 0 {
			dlStatus = dimStyle.Render(fmt.Sprintf("  DL: %d active, %d queued, %d done", st.Active, st.Queued, st.Completed))
			if st.Current != "" {
				dlStatus += dimStyle.Render("  " + st.Current)
			}
		} else if st.Completed > 0 || st.Failed > 0 {
			parts := []string{}
			if st.Completed > 0 {
				parts = append(parts, fmt.Sprintf("%d done", st.Completed))
			}
			if st.Failed > 0 {
				parts = append(parts, fmt.Sprintf("%d failed", st.Failed))
			}
			dlStatus = dimStyle.Render("  DL: " + strings.Join(parts, ", "))
		}
	}

	// --- Help bar / command line ---
	var help string
	if a.mode == modeCommand {
		help = "  " + a.cmdInput.View()
	} else {
		help = a.contextHelp()
	}

	// --- Separator lines ---
	separator := dimStyle.Render(strings.Repeat("─", a.width))

	// --- Footer (fixed bottom) ---
	// At small heights, skip download status to save space
	compact := a.height < 22
	var footerParts []string
	footerParts = append(footerParts, separator)
	if statusLine != "" {
		footerParts = append(footerParts, statusLine)
	}
	if dlStatus != "" && !compact {
		footerParts = append(footerParts, dlStatus)
	}
	footerParts = append(footerParts, np, help)
	footer := strings.Join(footerParts, "\n")

	// --- Fixed top (header + tab bar + separator) ---
	topFixed := lipgloss.JoinVertical(lipgloss.Left, "", header, tabBar, separator)

	// --- Calculate remaining content height ---
	topHeight := lipgloss.Height(topFixed)
	footerHeight := lipgloss.Height(footer)
	contentHeight := a.height - topHeight - footerHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	// --- Content area ---
	var content string
	switch {
	case a.mode == modeSavePlaylist:
		savePopup := overlayBorder.Padding(1, 2).Render(a.saveInput.View())
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, savePopup)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, savePopup,
				lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(bgColor)),
			)
		}

	case a.mode == modeRenamePlaylist:
		renamePopup := overlayBorder.Padding(1, 2).Render(a.renameInput.View())
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, renamePopup)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, renamePopup,
				lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(bgColor)),
			)
		}

	case a.mode == modeConfirmDelete:
		confirmContent := titleStyle.Render("Delete playlist?") + "\n\n" +
			normalStyle.Render(fmt.Sprintf("  \"%s\" will be permanently deleted.", a.deleteTarget)) + "\n\n" +
			dimStyle.Render("  y/enter confirm    n/esc cancel")
		confirmPopup := overlayBorder.Padding(1, 2).Render(confirmContent)
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, confirmPopup)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, confirmPopup,
				lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(bgColor)),
			)
		}

	case a.mode == modeAddToPlaylist:
		title := "Add to playlist"
		if a.addToTrack != nil {
			title = fmt.Sprintf("Add \"%s\" to:", a.addToTrack.Title)
		}
		var pickerContent string
		pickerContent = titleStyle.Render(title) + "\n\n"
		if a.addToCreating {
			pickerContent += "  " + a.saveInput.View() + "\n"
		} else {
			// "+ New playlist" option at index 0
			if a.addToPickerIdx == 0 {
				pickerContent += selectionStripe.Render("▸") + " " + titleStyle.Render("+ New playlist") + "\n"
			} else {
				pickerContent += "  " + dimStyle.Render("+ New playlist") + "\n"
			}
			for i, name := range a.addToPickerNames {
				if i+1 == a.addToPickerIdx {
					pickerContent += selectionStripe.Render("▸") + " " + normalStyle.Bold(true).Render(name) + "\n"
				} else {
					pickerContent += "  " + normalStyle.Render(name) + "\n"
				}
			}
		}
		addPopup := overlayBorder.Padding(1, 2).Render(pickerContent)
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, addPopup)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, addPopup,
				lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(bgColor)),
			)
		}

	case a.mode == modeHelp:
		helpOverlay := overlayBorder.
			Padding(1, 2).
			Render(
				titleStyle.Render("Keybindings") + "\n\n" +
					helpLine("tab/shift+tab", "Switch tabs") +
					helpLine("/", "Focus search") +
					helpLine("tab (in search)", "Toggle track/album/artist") +
					helpLine("enter", "Play track / browse album") +
					helpLine("esc", "Close overlay / cancel") +
					helpLine("backspace", "Back from album tracklist") +
					"\n" +
					helpLine("space", "Toggle pause") +
					helpLine("s", "Stop playback") +
					helpLine("n", "Next track in queue") +
					helpLine("p", "Previous track") +
					helpLine("a", "Queue track / append playlist") +
					helpLine("A", "Queue all album tracks") +
					helpLine("x", "Remove from queue / delete playlist") +
					helpLine("left/right", "Seek -5s / +5s") +
					helpLine("+/-", "Volume up/down") +
					"\n" +
					helpLine("j/k", "Navigate up/down") +
					helpLine("J/K", "Move queue track down/up") +
					helpLine("gg / G", "Jump to first / last item") +
					helpLine("home / end", "Jump to first / last item") +
					helpLine("ctrl+u/d", "Page up / page down") +
					helpLine("c", "Jump to now playing") +
					helpLine("f", "Filter queue") +
					helpLine("v / V", "Select track / select all") +
					"\n" +
					helpLine("l", "Toggle like") +
					helpLine("d / D", "Download track / album") +
					helpLine("b", "Play album from track") +
					helpLine("m", "More from this artist") +
					helpLine("M", "Similar artists") +
					helpLine("u", "Open album in browser") +
					helpLine("P", "Add track to playlist") +
					helpLine("S", "Save queue as playlist") +
					helpLine("R", "Toggle repeat") +
					helpLine("W", "Toggle track radio") +
					helpLine("Q", "Cycle audio quality") +
					helpLine("t", "Toggle elapsed/remaining") +
					helpLine(":", "Command mode") +
					helpLine("?", "Toggle this help") +
					helpLine("q / ctrl+c", "Quit"),
			)
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, helpOverlay)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, helpOverlay,
				lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(bgColor)),
			)
		}

	case a.searchVisible():
		searchOverlay := overlayBorder.
			Padding(1, 2).
			Width(a.width - 6).
			Render(a.search.View(a.width-12, a.likes.IsLiked, a.dlCheck(), spinner))
		popup := searchOverlay
		if errView != "" {
			popup = lipgloss.JoinVertical(lipgloss.Left, searchOverlay, errView)
		}
		if noColor {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, popup)
		} else {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, popup,
				lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(bgColor)),
			)
		}

	default:
		var tabContent string
		switch a.activeTab {
		case tabQueue:
			tabContent = a.renderQueueView()
		case tabRecent:
			tabContent = a.renderRecentView()
		case tabPlaylists:
			tabContent = a.renderPlaylistsView()
		}
		if errView != "" {
			tabContent = lipgloss.JoinVertical(lipgloss.Left, tabContent, errView)
		}
		content = lipgloss.NewStyle().Height(contentHeight).Render(tabContent)
	}

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, topFixed, content, footer))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (a App) contextHelp() string {
	switch a.mode {
	case modeSearchInput:
		return dimStyle.Render("  enter search  tab mode  ↑↓ history  esc close")
	case modeSearchBrowse:
		return dimStyle.Render("  enter select  a queue  d download  P playlist  S save album  esc close")
	case modeHelp:
		return dimStyle.Render("  esc close")
	case modeCommand:
		return dimStyle.Render("  :play liked|top|recent  :shuffle :radio :vol :save :load  enter run  esc cancel")
	case modeFilter:
		return dimStyle.Render("  type to filter  ↑↓ navigate  enter play  esc clear")
	case modeSavePlaylist:
		return dimStyle.Render("  enter save  esc cancel")
	case modeRenamePlaylist:
		return dimStyle.Render("  enter rename  esc cancel")
	case modeAddToPlaylist:
		return dimStyle.Render("  ↑↓ select  enter add  esc cancel")
	case modeConfirmDelete:
		return dimStyle.Render("  y/enter confirm  n/esc cancel")
	default:
		switch a.activeTab {
		case tabRecent:
			return dimStyle.Render("  enter play  a queue  P playlist  d download  / search  ? more  q quit")
		case tabPlaylists:
			return dimStyle.Render("  enter load  a append  r rename  x delete  / search  ? more  q quit")
		default:
			return dimStyle.Render("  enter play  x remove  S save  J/K move  / search  ? more  q quit")
		}
	}
}

func helpLine(key, desc string) string {
	return fmt.Sprintf("  %s  %s\n",
		titleStyle.Width(12).Render(key),
		desc,
	)
}
