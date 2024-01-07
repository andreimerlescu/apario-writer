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
	`fmt`
	`log`
	`os`
	`os/exec`
	`strconv`
	`strings`
)

func pp_save(pp PendingPage) {
	sm_pages.Store(pp.Identifier, pp)
	err := WritePendingPageToJson(pp)
	if err != nil {
		log.Printf("failed to write pending page %v to %v because of error %v", pp.Identifier, pp.ManifestPath, err)
	}
}

func parsePIDs(output string) []int {
	var pids []int

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			pid, err := strconv.Atoi(fields[1])
			if err == nil {
				pids = append(pids, pid)
			}
		}
	}

	return pids
}

func terminatePID(pid int) {
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error terminating PID", pid, ":", err)
	}
}
