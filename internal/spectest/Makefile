# Find all .wast files recursively in src/
WAST_FILES := $(shell find src -name '*.wast' -type f)

# Output directory base
OUT_DIR := compiled

# For each .wast file, create a corresponding directory path
# Example: src/foo/bar.wast -> compiled/foo/bar/
WAST_DIRS := $(patsubst src/%.wast,$(OUT_DIR)/%,$(WAST_FILES))

# Target JSON files
WAST_JSONS := $(addsuffix /wast.json,$(WAST_DIRS))

.PHONY: all clean

all: $(WAST_JSONS)

# Pattern rule: for each output JSON, derive the source .wast file and run wasm-tools
$(OUT_DIR)/%/wast.json: src/%.wast
	@mkdir -p $(dir $@)
	/Users/mpoindexter/dev/wasm-tools/target/debug/wasm-tools json-from-wast -vvv --pretty --wasm-dir $(dir $@) -o $@ $<

clean:
	rm -rf $(OUT_DIR)
