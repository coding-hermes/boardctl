package board

import "strings"

// PaperworkLaneDefault returns the priority a NEW task row defaults to when
// the board's lane is PAPERWORK: -review → P4, -docs / -readme → P5. Any
// non-paperwork lane returns "" and the caller keeps its own default.
//
// BT-071: the paperwork satellites (docs, readme, review) write reports, not
// code — real work, but never more urgent than the build/QA/PM rows — yet
// they filed at create's P2 default and crowded the top of every board view
// and the scheduler's pick order. The rule keys on the lane-CLASS suffix (the
// last hyphen-separated segment of the lane name, exact lowercase match),
// matching how the scheduler derives lane classes; there is deliberately no
// per-repo or per-name list. `-pm` and `-sync` are OPERATIONAL satellites,
// not paperwork, and do not count.
//
// The default applies to NEW writes only (create); update never invents a
// priority, and existing rows are never mass-rewritten by this rule.
func PaperworkLaneDefault(lane string) string {
	lane = strings.TrimSpace(lane)
	if i := strings.LastIndexByte(lane, '-'); i >= 0 {
		lane = lane[i+1:]
	}
	switch lane {
	case "review":
		return "P4"
	case "docs", "readme":
		return "P5"
	}
	return ""
}
