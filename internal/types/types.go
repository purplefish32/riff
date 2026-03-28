package types

type Artist struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type Album struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Cover       string `json:"cover"`
	ReleaseDate string `json:"releaseDate,omitempty"`
}

type AlbumFull struct {
	ID             int      `json:"id"`
	Title          string   `json:"title"`
	Cover          string   `json:"cover"`
	Duration       int      `json:"duration"`
	NumberOfTracks int      `json:"numberOfTracks"`
	ReleaseDate    string   `json:"releaseDate"`
	Artist         Artist   `json:"artist"`
	Artists        []Artist `json:"artists"`
}

type ArtistFull struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Picture    string `json:"picture"`
	Popularity float64 `json:"popularity"`
}

type ArtistSearchResponse struct {
	Data struct {
		Artists struct {
			Items []ArtistFull `json:"items"`
		} `json:"artists"`
	} `json:"data"`
}

// SimilarArtistsResponse is the response from /artist/similar/.
// Different structure from search: top-level "artists" array, not nested under "data".
type SimilarArtistsResponse struct {
	Artists []ArtistFull `json:"artists"`
}

type Track struct {
	ID           int      `json:"id"`
	Title        string   `json:"title"`
	Duration     int      `json:"duration"`
	TrackNumber  int      `json:"trackNumber"`
	Explicit     bool     `json:"explicit"`
	AudioQuality string   `json:"audioQuality"`
	URL          string   `json:"url"`
	Artist       Artist   `json:"artist"`
	Artists      []Artist `json:"artists"`
	Album        Album    `json:"album"`
}

type SearchResponse struct {
	Data struct {
		Limit              int     `json:"limit"`
		Offset             int     `json:"offset"`
		TotalNumberOfItems int     `json:"totalNumberOfItems"`
		Items              []Track `json:"items"`
	} `json:"data"`
}

type AlbumSearchResponse struct {
	Data struct {
		Albums struct {
			Items []AlbumFull `json:"items"`
		} `json:"albums"`
	} `json:"data"`
}

type AlbumTrackItem struct {
	Item Track  `json:"item"`
	Type string `json:"type"`
}

type AlbumResponse struct {
	Data struct {
		ID          int              `json:"id"`
		Title       string           `json:"title"`
		ReleaseDate string           `json:"releaseDate"`
		Artist      Artist           `json:"artist"`
		Items       []AlbumTrackItem `json:"items"`
	} `json:"data"`
}

type StreamResponse struct {
	Data struct {
		TrackID          int    `json:"trackId"`
		AudioQuality     string `json:"audioQuality"`
		ManifestMimeType string `json:"manifestMimeType"`
		Manifest         string `json:"manifest"`
		BitDepth         int    `json:"bitDepth"`
		SampleRate       int    `json:"sampleRate"`
	} `json:"data"`
}

type StreamManifest struct {
	MimeType       string   `json:"mimeType"`
	Codecs         string   `json:"codecs"`
	EncryptionType string   `json:"encryptionType"`
	URLs           []string `json:"urls"`
}
