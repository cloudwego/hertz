package docs

import _ "embed"

// filenames
const (
	guideENFile = "guide.md"
	guideCNFile = "guide_cn.md"
)

// file content
var (
	//go:embed guide.md
	guideEN string
	//go:embed guide_cn.md
	guideCN string
)

var GuideFiles = map[string]string{
	guideENFile: guideEN,
	guideCNFile: guideCN,
}
