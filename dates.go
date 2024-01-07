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
	`log`
	`strconv`
	`strings`
	`time`
)

func extractDates(in string) []time.Time {
	var dates []time.Time

	match1 := re_date1.FindAllStringSubmatch(in, -1)
	for _, m := range match1 {
		if len(m) < 4 {
			log.Printf("catch check m within match1 due to the length of m being %d", len(m))
			continue
		}
		day, dayErr := strconv.Atoi(m[1])
		if dayErr != nil {
			log.Printf("failed to parse the day %v inside the date %v with error %v", m[1], m, dayErr)
			continue
		}
		month := getMonthFromString(m[3])
		year, yearErr := strconv.Atoi(m[4])
		if yearErr != nil {
			log.Printf("failed to parse the year %v inside the date %v with error %v", m[2], m, yearErr)
			continue
		}
		dates = append(dates, time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
	}

	match2 := re_date2.FindAllStringSubmatch(in, -1)
	for _, m := range match2 {
		if len(m) < 3 {
			log.Printf("catch check m within match2 due to the length of m being %d", len(m))
			continue
		}
		month, _ := strconv.Atoi(m[1])
		day, dayErr := strconv.Atoi(m[2])
		if dayErr != nil {
			log.Printf("failed to parse the day %v inside the date %v with error %v", m[2], m, dayErr)
			continue
		}
		year, yearErr := strconv.Atoi(m[3])
		if yearErr != nil {
			log.Printf("failed to parse the year %v inside the date %v with error %v", m[3], m, yearErr)
			continue
		}
		dates = append(dates, time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC))
	}

	match3 := re_date3.FindAllStringSubmatch(in, -1)
	for _, m := range match3 {
		if len(m) < 2 {
			log.Printf("catch check m within match3 due to the length of m being %d", len(m))
			continue
		}
		month := getMonthFromString(m[1])
		year, yearErr := strconv.Atoi(m[2])
		if yearErr != nil {
			log.Printf("failed to parse the year %v inside the date %v with error %v", m[2], m, yearErr)
			continue
		}
		dates = append(dates, time.Date(year, month, 1, 0, 0, 0, 0, time.UTC))
	}

	match4 := re_date4.FindAllStringSubmatch(in, -1)
	for _, m := range match4 {
		if len(m) < 2 {
			log.Printf("catch check m within match6 due to the length of m being %d", len(m))
			continue
		}
		month := getMonthFromString(m[1])
		year, yearErr := strconv.Atoi(m[2])
		if yearErr != nil {
			log.Printf("failed to parse the year %v inside the date %v with error %v", m[2], m, yearErr)
			continue
		}
		dates = append(dates, time.Date(year, month, 1, 0, 0, 0, 0, time.UTC))
	}

	match5 := re_date5.FindAllStringSubmatch(in, -1)
	for _, m := range match5 {
		if len(m) < 3 {
			log.Printf("catch check m within match5 due to the length of m being %d", len(m))
			continue
		}
		month := getMonthFromString(m[1])
		day, dayErr := strconv.Atoi(m[2])
		if dayErr != nil {
			log.Printf("failed to parse the day %v inside the date %v with error %v", m[2], m, dayErr)
			continue
		}
		year, yearErr := strconv.Atoi(m[4])
		if yearErr != nil {
			log.Printf("failed to parse the year %v inside the date %v with error %v", m[3], m, yearErr)
			continue
		}
		dates = append(dates, time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
	}

	match6 := re_date6.FindAllStringSubmatch(in, -1)
	for _, m := range match6 {
		if len(m) < 1 {
			log.Printf("catch check m within match4 due to the length of m being %d", len(m))
			continue
		}
		year, yearErr := strconv.Atoi(m[1])
		if yearErr != nil {
			log.Printf("failed to parse the year %v inside the date %v with error %v", m[1], m, yearErr)
			continue
		}
		years := map[int]time.Time{}
		for _, date := range dates {
			years[date.Year()] = date
		}
		if _, found := years[year]; !found {
			dates = append(dates, time.Date(year, time.June, 14, 0, 0, 0, 0, time.UTC))
		}
	}

	return uniqueTimes(dates)
}

func getMonthFromString(monthStr string) time.Month {
	monthStr = strings.ToLower(monthStr)
	return m_months[monthStr]
}

func uniqueTimes(times []time.Time) []time.Time {
	seen := make(map[time.Time]bool)
	var unique []time.Time
	for _, t := range times {
		if t.Year() < 1800 || t.Year() > time.Now().Year() {
			continue
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		if _, ok := seen[t]; !ok {
			unique = append(unique, t)
			seen[t] = true
		}
	}
	return unique
}
