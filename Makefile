BIN := bin/mdcompress
CORPUS := never_commit/eval-corpus.jsonl
CORPUS_REPO_ROOT := /tmp/bench
EVAL_BACKEND := deepseek
EVAL_TIER := aggressive
EVAL_REPORT_MD := never_commit/eval-corpus-report.md
EVAL_REPORT_JSON := never_commit/eval-corpus-report.json
PER_RULE_REPORT_MD := never_commit/eval-rule-scoreboard.md
PER_RULE_REPORT_JSON := never_commit/eval-rule-scoreboard.json
EVAL_THRESHOLD := 0.90
WASM_DIR := dist/wasm

.PHONY: build wasm test eval eval-corpus eval-per-rule

build:
	go build -o $(BIN) ./cmd/mdcompress

# wasm builds the browser engine: mdcompress.wasm plus Go's wasm_exec.js
# runtime glue, written to $(WASM_DIR). The build is fully offline — token
# counts fall back to a len/4 estimate (no tokenizer tables embedded).
wasm:
	mkdir -p $(WASM_DIR)
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o $(WASM_DIR)/mdcompress.wasm ./cmd/mdcompress-wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_DIR)/wasm_exec.js
	@echo "wasm → $(WASM_DIR)/ ($$(ls -lh $(WASM_DIR)/mdcompress.wasm | awk '{print $$5}'))"

test:
	go test ./...

eval: eval-corpus eval-per-rule

eval-corpus: build
	./$(BIN) eval \
	  --corpus=$(CORPUS) \
	  --corpus-repo-root=$(CORPUS_REPO_ROOT) \
	  --backend=$(EVAL_BACKEND) \
	  --tier=$(EVAL_TIER) \
	  --threshold=$(EVAL_THRESHOLD) \
	  --markdown-out=$(EVAL_REPORT_MD) \
	  --json-out=$(EVAL_REPORT_JSON)

eval-per-rule: build
	./$(BIN) eval \
	  --corpus=$(CORPUS) \
	  --corpus-repo-root=$(CORPUS_REPO_ROOT) \
	  --backend=$(EVAL_BACKEND) \
	  --tier=$(EVAL_TIER) \
	  --per-rule-config \
	  --per-rule-out=$(PER_RULE_REPORT_MD) \
	  --per-rule-json=$(PER_RULE_REPORT_JSON)
