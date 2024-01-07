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
	`log`
)

func aggregatePendingPage(ctx context.Context, pp PendingPage) {
	if ch_CompiledDocument.CanWrite() {
		err := ch_CompiledDocument.Write(Document{
			Identifier:          pp.RecordIdentifier,
			URL:                 "",
			Pages:               nil,
			TotalPages:          0,
			CoverPageIdentifier: "",
			Collection:          Collection{},
		})
		if err != nil {
			log.Printf("cant write to the ch_CompiledDocument channel due to error %v", err)
			return
		}
	}
}
