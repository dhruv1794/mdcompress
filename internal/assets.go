package assets

import _ "embed"

//go:embed hooks/pre-commit.sh
var PreCommitHook string

//go:embed hooks/post-merge.sh
var PostMergeHook string

//go:embed skill/SKILL.md
var Skill string

//go:embed test.html
var TestPage string
