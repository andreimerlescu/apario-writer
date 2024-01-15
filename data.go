/*
Project Apario is the World's Truth Repository that was invented and started by Andrei Merlescu in 2020.
Copyright (C) 2023  Andrei Merlescu

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
package main

import (
	"context"
	"image/color"
	`regexp`
	"sync"
	"sync/atomic"
	"time"

	gem `github.com/andreimerlescu/go-gematria`
	sem `github.com/andreimerlescu/go-sema`
	sch `github.com/andreimerlescu/go-smartchan`
)

const (
	c_retry_attempts     = 33
	c_identifier_charset = "ABCDEFGHKMNPQRSTUVWXYZ123456789"
	c_dir_permissions    = 0111
)

var (
	startedAt = time.Now().UTC()

	// Integers
	channel_buffer_size    int = 1          // Buffered Channel's Size
	reader_buffer_bytes    int = 128 * 1024 // 128KB default buffer for reading CSV, XLSX, and PSV files into memory
	jpeg_compression_ratio     = 90         // Progressive JPEG Quality (valid options are 1-100)

	// Colors
	color_background = color.RGBA{R: 40, G: 40, B: 86, A: 255}    // navy blue
	color_text       = color.RGBA{R: 250, G: 226, B: 203, A: 255} // sky yellow

	// Strings
	dir_current_directory string

	// Maps
	m_cryptonyms        = make(map[string]string)
	m_used_identifiers  = make(map[string]bool)
	m_required_binaries = make(map[string]string)
	m_months            = map[string]time.Month{
		"jan": time.January, "january": time.January, "01": time.January, "1": time.January,
		"feb": time.February, "february": time.February, "02": time.February, "2": time.February,
		"mar": time.March, "march": time.March, "03": time.March, "3": time.March,
		"apr": time.April, "april": time.April, "04": time.April, "4": time.April,
		"may": time.May, "05": time.May, "5": time.May,
		"jun": time.June, "june": time.June, "06": time.June, "6": time.June,
		"jul": time.July, "july": time.July, "07": time.July, "7": time.July,
		"aug": time.August, "august": time.August, "08": time.August, "8": time.August,
		"sep": time.September, "september": time.September, "09": time.September, "9": time.September,
		"oct": time.October, "october": time.October, "10": time.October,
		"nov": time.November, "november": time.November, "11": time.November,
		"dec": time.December, "december": time.December, "12": time.December,
	}

	// Regex
	re_date1 = regexp.MustCompile(`(?i)(\d{1,2})(st|nd|rd|th)?\s(?:of\s)?(January|Jan|February|Feb|March|Mar|April|Apr|May|June|Jun|July|Jul|August|Aug|September|Sep|October|Oct|November|Nov|December|Dec),?\s(\d{2,4})`)
	re_date2 = regexp.MustCompile(`(?i)(\d{1,2})\/(\d{1,2})\/(\d{2,4})`)
	re_date3 = regexp.MustCompile(`(?i)(January|Jan|February|Feb|March|Mar|April|Apr|May|June|Jun|July|Jul|August|Aug|September|Sep|October|Oct|November|Nov|December|Dec),?\s(\d{2,4})`)
	re_date5 = regexp.MustCompile(`(?i)(January|Jan|February|Feb|March|Mar|April|Apr|May|June|Jun|July|Jul|August|Aug|September|Sep|October|Oct|November|Nov|December|Dec)\s(\d{1,2})(st|nd|rd|th)?,?\s(\d{2,4})`)
	re_date4 = regexp.MustCompile(`(?i)(January|Jan|February|Feb|March|Mar|April|Apr|May|June|Jun|July|Jul|August|Aug|September|Sep|October|Oct|November|Nov|December|Dec)\s(\d{4})`)
	re_date6 = regexp.MustCompile(`(\d{4})`)

	// Synchronization
	mu_identifier = sync.RWMutex{}
	//wg_active_tasks = cwg.CountableWaitGroup{}

	// Binary Dependencies
	sl_required_binaries = []string{
		"pdfcpu",
		"gs",
		"pdftotext",
		"convert",
		"pdftoppm",
		"tesseract",
		"clamscan",
	}
	sl_required_binaries_no_clam = []string{
		"pdfcpu",
		"gs",
		"pdftotext",
		"convert",
		"pdftoppm",
		"tesseract",
	}

	// Atomics
	a_i_total_pages        = atomic.Int64{}
	a_i_received_documents = atomic.Int32{}
	a_i_total_documents    = atomic.Int32{}

	// Concurrent Maps
	sm_page_directories sync.Map
	sm_resultdatas      sync.Map
	sm_documents        sync.Map
	sm_pages            sync.Map

	// Semaphores
	sem_tesseract  = sem.New(*flag_b_sem_tesseract)
	sem_download   = sem.New(*flag_b_sem_download)
	sem_pdfcpu     = sem.New(*flag_b_sem_pdfcpu)
	sem_gs         = sem.New(*flag_b_sem_gs)
	sem_pdftotext  = sem.New(*flag_b_sem_pdftotext)
	sem_convert    = sem.New(*flag_b_sem_convert)
	sem_pdftoppm   = sem.New(*flag_b_sem_pdftoppm)
	sem_png2jpg    = sem.New(*flag_g_sem_png2jpg)
	sem_resize     = sem.New(*flag_g_sem_resize)
	sem_shafile    = sem.New(*flag_g_sem_shafile)
	sema_watermark = sem.New(*flag_g_sem_watermark)
	sem_darkimage  = sem.New(*flag_g_sem_darkimage)
	sem_filedata   = sem.New(*flag_g_sem_filedata)
	sem_shastring  = sem.New(*flag_g_sem_shastring)
	sem_wjsonfile  = sem.New(*flag_g_sem_wjsonfile)

	// Channels
	ch_ImportedRow       = sch.NewSmartChan(channel_buffer_size)
	ch_ExtractText       = sch.NewSmartChan(channel_buffer_size)
	ch_ExtractPages      = sch.NewSmartChan(channel_buffer_size)
	ch_GeneratePng       = sch.NewSmartChan(channel_buffer_size)
	ch_GenerateLight     = sch.NewSmartChan(channel_buffer_size)
	ch_GenerateDark      = sch.NewSmartChan(channel_buffer_size)
	ch_ConvertToJpg      = sch.NewSmartChan(channel_buffer_size)
	ch_PerformOcr        = sch.NewSmartChan(channel_buffer_size)
	ch_AnalyzeText       = sch.NewSmartChan(channel_buffer_size)
	ch_AnalyzeCryptonyms = sch.NewSmartChan(channel_buffer_size)
	ch_CompletedPage     = sch.NewSmartChan(channel_buffer_size)
	ch_CompiledDocument  = sch.NewSmartChan(channel_buffer_size)
	ch_GenerateSocial    = sch.NewSmartChan(channel_buffer_size) // TODO: implement the ch_GenerateSocial channel
	ch_CompileDarkPDF    = sch.NewSmartChan(channel_buffer_size) // TODO: implement the ch_CompileDarkPDF channel
	ch_CompileSocialCard = sch.NewSmartChan(channel_buffer_size) // TODO: implement the ch_CompileSocialCard channel

	ch_Done = make(chan struct{}, 1)
)

type Document struct {
	Identifier          string         `json:"identifier"`
	URL                 string         `json:"url"`
	Pages               map[int64]Page `json:"pages"`
	TotalPages          int64          `json:"total_pages"`
	CoverPageIdentifier string         `json:"cover_page_identifier"`
	Collection          Collection     `json:"collection"`
	mu                  *sync.Mutex
}

type Page struct {
	Identifier         string            `json:"identifier"`
	DocumentIdentifier string            `json:"document_identifier"`
	PageNumber         int64             `json:"page_number"`
	Metadata           map[string]string `json:"metadata"`
	FullTextGematria   gem.Gematria      `json:"full_text_gematria"`
	FullText           string            `json:"full_text"`
}

type PDFCPUInfoResponseInfo struct {
	Source             string         `json:"source"`
	Version            string         `json:"version"`
	Pages              int            `json:"pages"`
	Title              string         `json:"title"`
	Author             string         `json:"author"`
	Subject            string         `json:"subject"`
	CreationDate       string         `json:"creationDate"`
	ModificationDate   string         `json:"modificationDate"`
	Keywords           []string       `json:"keywords"`
	Properties         map[string]any `json:"properties"`
	Tagged             bool           `json:"tagged"`
	Hybrid             bool           `json:"hybrid"`
	Linearized         bool           `json:"linearized"`
	UsingXRefStreams   bool           `json:"usingXRefStreams"`
	UsingObjectStreams bool           `json:"usingObjectStreams"`
	Watermarked        bool           `json:"watermarked"`
	Thumbnails         bool           `json:"thumbnails"`
	Form               bool           `json:"form"`
	Signatures         bool           `json:"signatures"`
	AppendOnly         bool           `json:"appendOnly"`
	Bookmarks          bool           `json:"bookmarks"`
	Names              bool           `json:"names"`
	Encrypted          bool           `json:"encrypted"`
	Permissions        int            `json:"permissions"`
}
type PDFCPUInfoResponse struct {
	Header map[string]string        `json:"header"`
	Infos  []PDFCPUInfoResponseInfo `json:"Infos"`
}

type Collection struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
}

type ResultData struct {
	Identifier        string                 `json:"identifier"`
	URL               string                 `json:"url"`
	DataDir           string                 `json:"data_dir"`
	PDFPath           string                 `json:"pdf_path"`
	URLChecksum       string                 `json:"url_checksum"`
	PDFChecksum       string                 `json:"pdf_checksum"`
	OCRTextPath       string                 `json:"ocr_text_path"`
	ExtractedTextPath string                 `json:"extracted_text_path"`
	RecordPath        string                 `json:"record_path"`
	TotalPages        int64                  `json:"total_pages"`
	Info              PDFCPUInfoResponseInfo `json:"info"`
	Metadata          map[string]string      `json:"metadata"`
}

type JPEG struct {
	Light Images `json:"light"`
	Dark  Images `json:"dark"`
}

type PNG struct {
	Light Images `json:"light"`
	Dark  Images `json:"dark"`
}

type PendingPage struct {
	Identifier       string      `json:"identifier"`
	RecordIdentifier string      `json:"record_identifier"`
	PageNumber       int         `json:"page_number"`
	PDFPath          string      `json:"pdf_path"`
	PagesDir         string      `json:"pages_dir"`
	OCRTextPath      string      `json:"ocr_text_path"`
	ManifestPath     string      `json:"manifest_path"`
	Language         string      `json:"language"`
	Cryptonyms       []string    `json:"cryptonyms"`
	Dates            []time.Time `json:"dates"`
	JPEG             JPEG        `json:"jpeg"`
	PNG              PNG         `json:"png"`
}

type Images struct {
	Original string `json:"original"`
	Large    string `json:"large"`
	Medium   string `json:"medium"`
	Small    string `json:"small"`
	Social   string `json:"social"`
}

type Column struct {
	Header string
	Value  string
}

type Qbit struct {
	seq   [3]byte
	count int
}

type CtxKey string
type CallbackFunc func(ctx context.Context, row []Column) error
