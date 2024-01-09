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
	"log"
	"os"
	"strings"
)

func analyze_StartOnFullText(ctx context.Context, pp PendingPage) {
	defer func() {
		pp_save(pp)
		if ch_AnalyzeCryptonyms.CanWrite() {
			err := ch_AnalyzeCryptonyms.Write(pp)
			if err != nil {
				log.Printf("cant write to the ch_AnalyzeCryptonyms channel due to error %v", err)
				return
			}
		}
	}()
	file, fileErr := os.ReadFile(pp.OCRTextPath)
	if fileErr != nil {
		log.Printf("Error opening file %q: %v\n", pp.OCRTextPath, fileErr)
		return
	}
	pp.Dates = extractDates(string(file))
}

func analyzeCryptonyms(ctx context.Context, pp PendingPage) {
	defer func() {
		pp_save(pp)
		if ch_CompletedPage.CanWrite() {
			err := ch_CompletedPage.Write(pp)
			if err != nil {
				log.Printf("cannot write to the ch_CompletedPage channel due to error %v", err)
				return
			}
		}
	}()

	var result []string
	file, fileErr := os.ReadFile(pp.OCRTextPath)
	if fileErr != nil {
		log.Printf("Error opening file %q: %v\n", pp.OCRTextPath, fileErr)
		return
	}
	for key := range m_cryptonyms {
		if strings.Contains(string(file), key) {
			result = append(result, key)
		}
	}
	pp.Cryptonyms = result
}
