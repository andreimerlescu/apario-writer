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
)

func aggregatePendingPage(ctx context.Context, pp PendingPage) {
	// will receive pending pages that are completed and the objective of this is to ensure that a map exists for that
	// document and all of the pages have been completed for processing;
	// once all pages have completed their processing, we need to compile the dark PDF of the entire document
	// then we need to perform OCR on the dark PDF and replace the non-OCR dark PDF with the OCR PDF
	// then we need to generate the social media share cards for the site + page with its metadata for SEO
	// then we need to verify the output completely and sign the created record

	data_rd, result_data_found := sm_resultdatas.Load(pp.RecordIdentifier)
	if !result_data_found {
		log.Printf("failed to find the document based on its identifier %d in the sm_resultdatas map", pp.RecordIdentifier)
		return
	}

	rd, rd_cast_ok := data_rd.(ResultData)
	if !rd_cast_ok {
		log.Printf("failed to typecast data_rd into ResultData")
		return
	}

	document_data, document_found := sm_documents.Load(pp.RecordIdentifier)
	if !document_found {
		log.Printf("failed to find the document based on its identifier %d in the sm_documents map", pp.RecordIdentifier)
		return
	}

	document, document_cast_ok := document_data.(Document)
	if !document_cast_ok {
		log.Printf("failed to typecast document_data into Document")
		return
	}

	if document.TotalPages != rd.TotalPages {
		log.Printf("document.TotalPages [%d] != [%d] rd.TotalPages", document.TotalPages, rd.TotalPages)
	}

	if document.mu == nil {
		log.Printf("document is missing a mutex defined on it in aggregatePendingPage for Document ID: %s",
			document.Identifier)
		return
	}

	document.mu.Lock()
	if document.Pages == nil {
		document.Pages = make(map[int64]Page)
	}
	document.Pages[int64(pp.PageNumber)] = Page{
		Identifier:         pp.Identifier,
		DocumentIdentifier: pp.RecordIdentifier,
		PageNumber:         int64(pp.PageNumber),
	}
	document.mu.Unlock()

	if int64(len(document.Pages)) == document.TotalPages {
		if ch_CompiledDocument.CanWrite() {
			err := ch_CompiledDocument.Write(document)
			if err != nil {
				log.Printf("cant write to the ch_CompiledDocument channel due to error %v", err)
				return
			}
		} else {
			log.Printf("cannot write to the ch_CompiledDocument after the compilation of document %v completed",
				document.Identifier)
		}
	} else {
		log.Printf("aggregatePendingPage document %v page %d received but the document.Pages are at %d of %d so waiting before sending into ch_CompiledDocument",
			pp.RecordIdentifier, pp.PageNumber, len(document.Pages), document.TotalPages)
	}

}
