package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

//go:embed LICENSE
var fs_license embed.FS

//go:embed bundled/*
var fs_references embed.FS

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	for i, arg := range os.Args {
		if arg == "help" {
			fmt.Println(config.Usage())
			os.Exit(0)
		}
		if arg == "show" {
			for _, innerArg := range os.Args {
				if innerArg == "w" || innerArg == "c" {
					license, err := fs_license.ReadFile("LICENSE")
					if err != nil {
						fmt.Printf("Cannot find the license file to load to comply with the GNU-3 license terms. This program was modified outside of its intended runtime use.")
						os.Exit(1)
					} else {
						fmt.Printf("%v\n", string(license))
						os.Exit(1)
					}
				}
			}
		}
		if strings.HasPrefix(arg, `-`) && strings.HasSuffix(arg, "config") {
			if len(os.Args) >= i+1 {
				arg_config_yaml = strings.Clone(os.Args[i+1])
			}
		}
	}

	// Attempt to read from the `--config` as a file, default: config.yaml
	var configErr error
	var configStatErr error
	var configFile string
	if len(arg_config_yaml) > 0 {
		_, configStatErr = os.Stat(arg_config_yaml)
		if configStatErr == nil || !errors.Is(configStatErr, os.ErrNotExist) {
			configFile = strings.Clone(arg_config_yaml)
		}
	} else {
		if len(*flag_s_config_file) == 0 {
			_, configStatErr = os.Stat(filepath.Join(".", "config.yaml"))
			if configStatErr == nil || !errors.Is(configStatErr, os.ErrNotExist) {
				configFile = filepath.Join(".", "config.yaml")
			}
		} else {
			_, configStatErr = os.Stat(*flag_s_config_file)
			if configStatErr == nil || !errors.Is(configStatErr, os.ErrNotExist) {
				configFile = strings.Clone(*flag_s_config_file)
			}
		}
	}

	configErr = config.Parse(configFile)
	if configErr != nil {
		log.Fatalf("failed to parse config file: %v", configErr)
	}

	var binaryErr error
	if *flag_b_disable_clamav {
		binaryErr = verifyBinaries(sl_required_binaries_no_clam)
	} else {
		binaryErr = verifyBinaries(sl_required_binaries)
	}

	if binaryErr != nil {
		fmt.Printf("Error: %s\n", binaryErr)
		os.Exit(1)
	}

	ex, execErr := os.Getwd()
	if execErr != nil {
		panic(execErr)
	}

	dir_current_directory = filepath.Dir(ex)
	_ = fmt.Sprintf("Current Working Directory: %s\n", dir_current_directory)

	if *flag_s_download_pdf_url == "" && *flag_s_import_pdf_path == "" &&
		*flag_s_import_directory == "" && *flag_s_import_csv == "" /* && *flag_s_import_xlsx == ""  */ {
		flag.Usage()
		log.Printf("You must use one --download-pdf-url / --import-pdf-path / --import-directory / --import-csv")
		//log.Printf("You must use one --download-pdf-url / --import-pdf-path / --import-directory / --import-csv / --import-xlsx")
		os.Exit(1)
	}

	if (*flag_s_download_pdf_url != "" || *flag_s_import_pdf_path != "") &&
		*flag_s_pdf_title == "" {
		flag.Usage()
		log.Printf("Cannot use --download-pdf-url or --import-pdf-path without --pdf-title")
		os.Exit(1)
	}

	if *flag_s_database_directory == "" || !IsDir(*flag_s_database_directory) {
		flag.Usage()
		log.Printf("You are required to specify the apario-reader database path with --database-directory")
		os.Exit(1)
	}

	if *flag_i_sem_limiter > 0 {
		channel_buffer_size = *flag_i_sem_limiter
	}

	if *flag_i_buffer > 0 {
		reader_buffer_bytes = *flag_i_buffer
	}

	// Use a configurable to send logs to a specific file
	logDir := filepath.Dir(*flag_g_log_file)
	_, ldiErr := os.Stat(logDir)
	if ldiErr != nil && errors.Is(ldiErr, os.ErrNotExist) {
		mkErr := os.MkdirAll(logDir, 0755)
		if mkErr != nil {
			log.Panicf("failed to open file %v due to %+v", *flag_g_log_file, ldiErr)
		}
	}
	logFile, logFileErr := os.OpenFile(*flag_g_log_file, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if logFileErr != nil {
		log.Fatal("Failed to open log file: ", logFileErr)
	}
	log.SetOutput(logFile) // redirect log.* to the log file versus fmt to STDOUT

	// Check for cleanup failure from previous run
	cleanupFailedPath := filepath.Join(logDir, ".cleanup_failed")
	if _, err := os.Stat(cleanupFailedPath); err == nil {
		// Cleanup failed file exists, perform rotation
		if err := rotate_log_files(logDir); err != nil {
			log.Fatalf("failed to rotate logs after cleanup failure: %v", err)
		}
		// Remove the cleanup failed file
		_ = os.Remove(cleanupFailedPath)
	}

	log_files = make(map[string]*os.File)

	// Initialize log files with truncation
	debugFile, err := os.OpenFile(filepath.Join(logDir, "debug.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("failed to open debug log: %v", err)
	}
	log_files[cDebugLog] = debugFile

	infoFile, err := os.OpenFile(filepath.Join(logDir, "info.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		closeLogFiles()
		log.Fatalf("failed to open info log: %v", err)
	}
	log_files[cInfoLog] = infoFile

	errorFile, err := os.OpenFile(filepath.Join(logDir, "error.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		closeLogFiles()
		log.Fatalf("failed to open error log: %v", err)
	}
	log_files[cErrorLog] = errorFile

	// Initialize loggers
	log_debug = NewCustomLogger(debugFile, "DEBUG: ", log.Ldate|log.Ltime|log.Llongfile, 10)
	log_info = NewCustomLogger(infoFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile, 10)
	log_error = NewCustomLogger(errorFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile, 10)

	// interrupt Ctrl+C and other SIGINT/SIGTERM/SIGKILL related signals to the application to quit gracefully
	watchdog := make(chan os.Signal, 1)
	signal.Notify(watchdog, os.Kill, syscall.SIGTERM, os.Interrupt)
	go receive_watchdog_signal(watchdog, logFile, cancel)

	// process/analyze the cryptonyms from the bundled assets
	cryptonymFile, cryptonymFileErr := fs_references.ReadFile(filepath.Join("bundled", "reference", "cryptonyms.json"))
	if cryptonymFileErr != nil {
		log_error.Printf("failed to parse cryptonyms.json file from the data directory due to error: %v", cryptonymFileErr)
	} else {
		cryptonymMarshalErr := json.Unmarshal(cryptonymFile, &m_cryptonyms)
		if cryptonymMarshalErr != nil {
			log_error.Printf("failed to load the m_cryptonyms due to error %v", cryptonymMarshalErr)
		}
		out := ""
		var cryptonyms []string
		for cryptonym, _ := range m_cryptonyms {
			cryptonyms = append(cryptonyms, cryptonym)
		}
		out = strings.Join(cryptonyms, ",")
		log_info.Printf("Cryptonyms to search for: %v", out)
	}

	// which action are we doing?
	if *flag_s_download_pdf_url != "" && *flag_s_import_pdf_path != "" {
		flag.Usage()
		log_error.Printf("Cannot use --download-pdf-url with --import-pdf-path.")
		os.Exit(1)
	} else if *flag_s_download_pdf_url != "" && *flag_s_import_directory != "" {
		flag.Usage()
		log_error.Printf("Cannot use --download-pdf-url with --import-directory.")
		os.Exit(1)
	} else if *flag_s_import_pdf_path != "" && *flag_s_import_directory != "" {
		flag.Usage()
		log_error.Printf("Cannot use --import-pdf-path with --import-directory.")
		os.Exit(1)
	} // TODO: add the xlsx and csv options

	// store the filename of what is being processed into a variable
	filename := *flag_s_download_pdf_url // default
	if *flag_s_import_pdf_path != "" {
		filename = *flag_s_import_pdf_path
	}
	if *flag_s_import_directory != "" {
		filename = *flag_s_import_directory
	}

	// TODO: add conditionals for filename for xlsx and csv options

	// attach the filename to the context so it can be observed from within the goroutines of the main processor
	ctx = context.WithValue(ctx, CtxKey("filename"), filename)

	// start a bunch of receiver functions to handle when data is ready to be processed
	// each of these functions are like black boxes that ONE PAGE from a document is ingested into until it reaches the end
	go receiveImportedRow(ctx, ch_ImportedRow.Chan())            // step 01 - runs validate_result_data_record before sending into ch_ExtractText
	go receiveOnExtractTextCh(ctx, ch_ExtractText.Chan())        // step 02 - runs extractPlainTextFromPdf before sending into ch_ExtractPages
	go receiveOnExtractPagesCh(ctx, ch_ExtractPages.Chan())      // step 03 - runs extractPagesFromPdf before sending PendingPage into ch_GeneratePng
	go receiveOnGeneratePngCh(ctx, ch_GeneratePng.Chan())        // step 04 - runs convertPageToPng before sending PendingPage into ch_GenerateLight
	go receiveOnGenerateLightCh(ctx, ch_GenerateLight.Chan())    // step 05 - runs generateLightThumbnails before sending PendingPage into ch_GenerateDark
	go receiveOnGenerateDarkCh(ctx, ch_GenerateDark.Chan())      // step 06 - runs generateDarkThumbnails before sending PendingPage into ch_ConvertToJpg
	go receiveOnConvertToJpg(ctx, ch_ConvertToJpg.Chan())        // step 07 - runs convertPngToJpg before sending PendingPage into ch_PerformOcr
	go receiveOnPerformOcrCh(ctx, ch_PerformOcr.Chan())          // step 08 - runs performOcrOnPdf before sending PendingPage into ch_AnalyzeText
	go receiveFullTextToAnalyze(ctx, ch_AnalyzeText.Chan())      // step 09 - runs analyze_StartOnFullText before sending PendingPage into ch_AnalyzeCryptonyms
	go receiveAnalyzeCryptonym(ctx, ch_AnalyzeCryptonyms.Chan()) // step 10 - runs analyzeCryptonyms before sending PendingPage into ch_CompletedPage
	go receiveCompletedPendingPage(ctx, ch_CompletedPage.Chan()) // step 11 - performs a sanity check on the compilation

	var importErr error
	if *flag_s_download_pdf_url != "" {
		importErr = process_download_pdf(ctx, *flag_s_download_pdf_url, *flag_s_pdf_metadata_json)
	} else if *flag_s_import_pdf_path != "" {
		importErr = process_import_pdf(ctx, *flag_s_import_pdf_path, *flag_s_pdf_metadata_json)
	} else if *flag_s_import_directory != "" {
		importErr = process_import_directory(ctx, *flag_s_import_directory)
	} else if *flag_s_import_csv != "" {
		importErr = process_import_csv(ctx, *flag_s_import_csv, process_custom_csv_row)
	} else if *flag_s_import_xlsx != "" {
		importErr = process_import_xlsx(ctx, *flag_s_import_xlsx, processRecord)
	} else {
		panic("Improperly formatted configuration. No data to process.")
	}

	if importErr != nil {
		log_error.Printf("received an error from process_import_csv/process_import_xlsx namely: %v", importErr) // a problem habbened
	}

	defer func(logFile *os.File) {
		err := logFile.Close()
		if err != nil {
			log_error.Printf("failed to close the logfile due to err: %v", err)
		}
	}(logFile)

	for {
		select {
		case <-ctx.Done():
			elapsed := time.Since(startedAt)
			log.Printf("Completed task in %.0f seconds", elapsed.Seconds())
			return
		case <-ch_Done:
			log.SetOutput(os.Stdout)
			log.Printf("done processing everything... time to end things now!")
			watchdog <- os.Kill
		case id, ok := <-ch_CompiledDocument.Chan():
			if ok {
				d, ok := id.(Document)
				if !ok {
					log_error.Printf("cannot typecast the final result for %s as a .(Document)", d.Identifier)
				}
				a_i_received_documents.Add(1)
				log_info.Printf("a_i_total_documents == a_i_received_documents ; %d == %d",
					a_i_total_documents.Load(), a_i_received_documents.Load())

				if a_i_total_documents.Load() == a_i_received_documents.Load() {
					log.SetOutput(os.Stdout)
					log.Printf("Completed processing document %v", d)
					ch_Done <- struct{}{}
				}
			}
		}
	}
}

func receive_watchdog_signal(watchdog chan os.Signal, logFile *os.File, cancel context.CancelFunc) {
	<-watchdog
	log.SetOutput(os.Stdout)
	err := logFile.Close()
	if err != nil {
		log_error.Printf("failed to close the logFile due to error: %v", err)
	}
	defer cancel()

	ch_ImportedRow.Close()       // step 01
	ch_ExtractText.Close()       // step 02
	ch_ExtractPages.Close()      // step 03
	ch_GeneratePng.Close()       // step 04
	ch_GenerateLight.Close()     // step 05
	ch_GenerateDark.Close()      // step 06
	ch_ConvertToJpg.Close()      // step 07
	ch_PerformOcr.Close()        // step 08
	ch_AnalyzeText.Close()       // step 09
	ch_AnalyzeCryptonyms.Close() // step 10
	ch_CompletedPage.Close()     // step 11
	ch_CompiledDocument.Close()  // step 12

	fmt.Printf("Completed running in %d", time.Since(startedAt))

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq apario-writer.exe")
	default:
		cmd = exec.Command("pgrep", "apario-writer")
	}

	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	pids := parsePIDs(string(output))

	for _, pid := range pids {
		terminatePID(pid)
	}

}

func rotate_log_files(logDir string) error {
	timestamp := time.Now().UTC().Format(FileFullTimeFormat)

	// List of files to rotate
	files := []string{cInfoLog, cDebugLog, cErrorLog}

	for _, filename := range files {
		oldPath := filepath.Join(logDir, filename)
		newPath := filepath.Join(logDir, fmt.Sprintf("%s.%s", timestamp, filename))

		// Check if old file exists before attempting rotation
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				// Create cleanup failed marker
				cleanupFailedFile := filepath.Join(logDir, ".cleanup_failed")
				_ = os.WriteFile(cleanupFailedFile, []byte("Cleanup failed during log rotation"), 0644)
				return fmt.Errorf("failed to rotate %s: %v", filename, err)
			}
		}
	}

	return nil
}

func closeLogFiles() {
	for name, logFile := range log_files {
		err := logFile.Close()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "closeLogFiles %s caused err: %+v\n", name, err)
		}
	}
}
