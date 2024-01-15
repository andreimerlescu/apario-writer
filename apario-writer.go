package main

import (
	`context`
	`embed`
	`encoding/json`
	`flag`
	`fmt`
	`log`
	`os`
	`os/exec`
	`os/signal`
	`path/filepath`
	`runtime`
	`strings`
	`syscall`
	`time`
)

//go:embed LICENSE
var fs_license embed.FS

//go:embed bundled/*
var fs_references embed.FS

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	for _, arg := range os.Args {
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
	}

	// Attempt to read from the `--config` as a file, default: config.yaml
	configErr := config.Parse(*flag_s_config_file)
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
	logFile, logFileErr := os.OpenFile(*flag_g_log_file, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if logFileErr != nil {
		log.Fatal("Failed to open log file: ", logFileErr)
	}
	log.SetOutput(logFile) // redirect log.* to the log file versus fmt to STDOUT

	// interrupt Ctrl+C and other SIGINT/SIGTERM/SIGKILL related signals to the application to quit gracefully
	watchdog := make(chan os.Signal, 1)
	signal.Notify(watchdog, os.Kill, syscall.SIGTERM, os.Interrupt)
	go receive_watchdog_signal(watchdog, logFile, cancel)

	// process/analyze the cryptonyms from the bundled assets
	cryptonymFile, cryptonymFileErr := fs_references.ReadFile(filepath.Join("bundled", "reference", "cryptonyms.json"))
	if cryptonymFileErr != nil {
		log.Printf("failed to parse cryptonyms.json file from the data directory due to error: %v", cryptonymFileErr)
	} else {
		cryptonymMarshalErr := json.Unmarshal(cryptonymFile, &m_cryptonyms)
		if cryptonymMarshalErr != nil {
			log.Printf("failed to load the m_cryptonyms due to error %v", cryptonymMarshalErr)
		}
		out := ""
		var cryptonyms []string
		for cryptonym, _ := range m_cryptonyms {
			cryptonyms = append(cryptonyms, cryptonym)
		}
		out = strings.Join(cryptonyms, ",")
		log.Printf("Cryptonyms to search for: %v", out)
	}

	// which action are we doing?
	if *flag_s_download_pdf_url != "" && *flag_s_import_pdf_path != "" {
		flag.Usage()
		log.Printf("Cannot use --download-pdf-url with --import-pdf-path.")
		os.Exit(1)
	} else if *flag_s_download_pdf_url != "" && *flag_s_import_directory != "" {
		flag.Usage()
		log.Printf("Cannot use --download-pdf-url with --import-directory.")
		os.Exit(1)
	} else if *flag_s_import_pdf_path != "" && *flag_s_import_directory != "" {
		flag.Usage()
		log.Printf("Cannot use --import-pdf-path with --import-directory.")
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
	go receiveImportedRow(ctx, ch_ImportedRow.Chan())            // step 01 - runs validatePdf before sending into ch_ExtractText
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
		log.Printf("process_download_pdf")
		importErr = process_download_pdf(ctx, *flag_s_download_pdf_url, *flag_s_pdf_metadata_json)
	} else if *flag_s_import_pdf_path != "" {
		log.Printf("process_import_pdf")
		importErr = process_import_pdf(ctx, *flag_s_import_pdf_path, *flag_s_pdf_metadata_json)
	} else if *flag_s_import_directory != "" {
		log.Printf("process_import_directory")
		importErr = process_import_directory(ctx, *flag_s_import_directory)
	} else if *flag_s_import_csv != "" {
		log.Printf("process_import_csv")
		importErr = process_import_csv(ctx, *flag_s_import_csv, process_custom_csv_row)
	} else if *flag_s_import_xlsx != "" {
		log.Printf("process_import_xlsx")
		importErr = process_import_xlsx(ctx, *flag_s_import_xlsx, processRecord)
	} else {
		panic("Improperly formatted configuration. No data to process.")
	}

	if importErr != nil {
		log.Printf("received an error from process_import_csv/process_import_xlsx namely: %v", importErr) // a problem habbened
	}

	defer func(logFile *os.File) {
		err := logFile.Close()
		if err != nil {
			log.Printf("failed to close the logfile due to err: %v", err)
		}
	}(logFile)

	for {
		select {
		case <-ctx.Done():
			log.Printf("received a cancel on the context's done channel")
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
					log.Printf("cannot typecast the final result for %s as a .(Document)", d.Identifier)
				}
				a_i_received_documents.Add(1)
				log.Printf("a_i_total_documents == a_i_received_documents ; %d == %d", a_i_total_documents.Load(), a_i_received_documents.Load())

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
		log.Printf("failed to close the logFile due to error: %v", err)
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
