file="examples/all.go"

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# CAUGHT / NOT CAUGHT / UNEXPECTED
(
  # Extract only line numbers from CATCH comments, ensure sorted (lexicographic for comm)
  rg -n "// CATCH" "$file" | cut -d: -f1 | LC_ALL=C sort | uniq > /tmp/catch_lines.txt

  # Extract only line numbers from linter (ignore column and message), ensure sorted
  # Note: linter outputs to stderr, so we need 2>&1 to capture it
  ./pls-dont-go "$file" 2>&1 | grep "$file" | cut -d: -f2 | LC_ALL=C sort | uniq > /tmp/linter_hits.txt

  # CAUGHT
  LC_ALL=C comm -12 /tmp/catch_lines.txt /tmp/linter_hits.txt | while read l; do
    printf "%-30s ${GREEN}[CAUGHT]${NC}\n" "$file:$l"
  done

  # NOT CAUGHT
  LC_ALL=C comm -23 /tmp/catch_lines.txt /tmp/linter_hits.txt | while read l; do
    # Extract the line content
    line_content=$(sed -n "${l}p" "$file")
    
    # Extract the reason after "// CATCH -" if it exists
    reason=$(echo "$line_content" | sed -n 's/.*\/\/ CATCH - \(.*\)/\1/p')
    
    if [ -n "$reason" ]; then
      printf "%-30s ${RED}[NOT CAUGHT]${NC} %s\n" "$file:$l" "$reason"
    else
      printf "%-30s ${RED}[NOT CAUGHT]${NC}\n" "$file:$l"
    fi
  done

  # UNEXPECTED
  LC_ALL=C comm -13 /tmp/catch_lines.txt /tmp/linter_hits.txt | while read l; do
    printf "%-30s ${YELLOW}[UNEXPECTED]${NC}\n" "$file:$l"
  done

  # Statistics
  echo ""
  caught_count=$(LC_ALL=C comm -12 /tmp/catch_lines.txt /tmp/linter_hits.txt | wc -l)
  not_caught_count=$(LC_ALL=C comm -23 /tmp/catch_lines.txt /tmp/linter_hits.txt | wc -l)
  unexpected_count=$(LC_ALL=C comm -13 /tmp/catch_lines.txt /tmp/linter_hits.txt | wc -l)
  total_expected=$(wc -l < /tmp/catch_lines.txt)
  
  printf "%-15s %s\n" "caught:" "$caught_count/$total_expected"
  printf "%-15s %s\n" "not caught:" "$not_caught_count/$total_expected"
  printf "%-15s %s\n" "unexpected:" "$unexpected_count"
)
