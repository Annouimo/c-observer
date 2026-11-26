package engine

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type sourceOrigin struct {
	offset        int
	file, logical string
	line          int
}

type sourceOrigins []sourceOrigin

var lineDirective = regexp.MustCompile(`^\s*#\s*line\s+([0-9]+)\s+("(?:[^"\\]|\\.)*")\s*$`)
var otherLineDirective = regexp.MustCompile(`^\s*#\s*(?:line\b|[0-9])`)

// Clang omits repeated presumed locations from JSON. Track physical lines and
// accept a logical origin only after matching the frozen user source text.
func buildSourceOrigins(compiled, dir string, files, sourceMap map[string]string) sourceOrigins {
	if len(sourceMap) == 0 {
		return nil
	}
	originals := make(map[string][]string)
	var origins sourceOrigins
	var logical, file string
	line, offset := 0, 0
	for _, raw := range strings.Split(compiled, "\n") {
		text := strings.TrimSuffix(raw, "\r")
		origin := sourceOrigin{offset: offset}
		if match := lineDirective.FindStringSubmatch(text); match != nil {
			logical, _ = strconv.Unquote(match[2])
			line, _ = strconv.Atoi(match[1])
			file = ""
			if !strings.HasPrefix(strings.ReplaceAll(logical, "\\", "/"), "preset/") {
				target := sourceMap[logical]
				if target == "" {
					target = sourceMap[absolute(dir, logical)]
				}
				if target != "" {
					target = absolute(dir, target)
					if frozen, ok := files[target]; ok {
						file = target
						if _, exists := originals[file]; !exists {
							originals[file] = strings.Split(frozen, "\n")
						}
					}
				}
			}
		} else if otherLineDirective.MatchString(text) {
			file = ""
		} else {
			if source := originals[file]; line > 0 && line <= len(source) &&
				text == strings.TrimSuffix(source[line-1], "\r") {
				origin.file, origin.logical, origin.line = file, logical, line
			}
			line++
		}
		origins = append(origins, origin)
		offset += len(raw) + 1
	}
	return origins
}

func (origins sourceOrigins) resolve(offset int, presumedFile string, presumedLine int) (string, int) {
	i := sort.Search(len(origins), func(i int) bool { return origins[i].offset > offset }) - 1
	if i < 0 {
		return "", 0
	}
	origin := origins[i]
	if presumedFile != "" && presumedFile != origin.logical || presumedLine != 0 && presumedLine != origin.line {
		return "", 0
	}
	return origin.file, origin.line
}
