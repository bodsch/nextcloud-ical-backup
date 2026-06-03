package service

import "bodsch.me/nextcloud-ical-backup/internal/util"

// BuildVCard concatenates raw contact cards into a single vCard line list.
// Each card is already a complete BEGIN:VCARD … END:VCARD block, so they only
// need to be normalized and joined.
func BuildVCard(rawCards []string) []string {
	var lines []string
	for _, raw := range rawCards {
		lines = append(lines, util.NormalizeLines(raw)...)
	}
	return lines
}
