package main

import (
	"fmt"
	"os"
	"strings"
)

type token_t struct {
	tok_type string
	tok_raw  string
}

type markdownParseNode struct {
	idx1     int
	idx2     int
	depth    int
	property string
}

type MarkdownState struct {
	buf            []token_t
	filepath       string
	filedata       [][]byte
	fullfileoffset int
	bufsize        int
	row            int
	col            int

	parseIndex int

	done int

	nodes []markdownParseNode

	renderedText string
}

// These really only apply to lexical analysis.
var _MDParser_ExitCodeMap map[int]string = map[int]string{
	100: "E_OK",
	200: "EOF",
	300: "ColumnOverflow",
	400: "MissingFile",
	500: "Invalid File",
	600: "Missing/Invalid data",
}

var _MDParser_Tokens map[string]string = map[string]string{
	"#":  "HEADER",
	"(":  "ENCLOSER",
	")":  "ENCLOSER",
	"[":  "ENCLOSER",
	"]":  "ENCLOSER",
	"=":  "PUNCTUATOR",
	"-":  "PUNCTUATOR",
	"*":  "ENCLOSER",
	"~":  "ENCLOSER",
	"\n": "NEWLINE",
}

func outputParserError(errno int, filepath string, row int, col int) {
	result := fmt.Sprintf(
		"Parsing error \"%s\" found at %d:%d in %s",
		_MDParser_ExitCodeMap[errno],
		row,
		col,
		filepath,
	)
	fmt.Fprintln(os.Stderr, result)
}

func LoadFile(md *MarkdownState, filepath string) error {
	md.filepath = filepath

	s := strings.Split(md.filepath, ".")
	if s[len(s)-1] != "md" {
		md.done = 500
		outputParserError(md.done, md.filepath, md.row, md.col)
		return nil
	}

	dat, err := os.ReadFile(md.filepath)
	if err != nil {
		return err
	}

	datArr := []byte(string(dat))
	md.bufsize = len(datArr)

	fileBuffer := [][]byte{}

	firstIdx := 0
	secondIdx := 0
	for i := range string(datArr) {
		if byte(datArr[i]) == byte('\n') {
			secondIdx = i
			fileBuffer = append(fileBuffer, datArr[firstIdx:secondIdx])
			firstIdx = i
		}
	}

	md.filedata = fileBuffer

	// adding padding
	md.bufsize++
	md.filedata = append(md.filedata, []byte(" "))

	md.done = 100
	return nil
}

func next(md *MarkdownState) {
	if md.fullfileoffset >= md.bufsize-1 {
		md.done = 200
		return
	} else if md.col == len(md.filedata[md.row])-1 {
		md.row++
		md.col = 0
	} else {
		md.col++
	}
	md.done = 100
	md.fullfileoffset++
	return
}

func PopulateMarkdownStateBuffer(md *MarkdownState) {
	for _ = range md.bufsize - 1 {
		currentChar := string(md.filedata[md.row][md.col])
		if len(_MDParser_Tokens[currentChar]) > 0 {
			t := token_t{
				tok_type: _MDParser_Tokens[currentChar],
				tok_raw:  currentChar,
			}
			md.buf = append(md.buf, t)
		} else {
			t := token_t{
				tok_type: "LITERAL",
				tok_raw:  currentChar,
			}
			md.buf = append(md.buf, t)
		}
		next(md)
	}
}

func headerParseRule(md *MarkdownState) []markdownParseNode {
	// check for no header
	indexBuffer := []markdownParseNode{}

	i := md.parseIndex
	for i < len(md.buf) {
		if md.buf[i].tok_raw == "#" && md.buf[i+1].tok_raw != "#" {
			j := i + 1
			for j < len(md.buf) {
				if md.buf[j].tok_type == "NEWLINE" {
					tmp := markdownParseNode{
						idx1:     i + 2,
						idx2:     j,
						property: "HEADING",
						depth:    0,
					}

					fillerToken := token_t{
						tok_raw:  " ",
						tok_type: "LITERAL",
					}

					md.buf[i] = fillerToken
					md.buf[i+1] = fillerToken

					//fmt.Println(md.buf)
					indexBuffer = append(indexBuffer, tmp)
					//indexBuffer = append(indexBuffer, []int{i + 2, j})
					break
				}
				j++
			}
		} else if md.buf[i].tok_raw == "#" && md.buf[i+1].tok_raw == "#" {
			j := i + 1
			for j < len(md.buf) {
				if md.buf[j].tok_type == "NEWLINE" {

					tmp := markdownParseNode{
						idx1:     i + 3,
						idx2:     j,
						property: "HEADING",
						depth:    1,
					}

					fillerToken := token_t{
						tok_raw:  " ",
						tok_type: "LITERAL",
					}

					md.buf[i] = fillerToken
					md.buf[i+1] = fillerToken

					indexBuffer = append(indexBuffer, tmp)
					//indexBuffer = append(indexBuffer, []int{i + 3, j})
					break
				}
				j++
			}
		}
		i++
	}
	if i > md.parseIndex {
		md.parseIndex = i
	}
	md.parseIndex = 0
	return indexBuffer
}

func italicParseRule(md *MarkdownState) []markdownParseNode {
	indexBuffer := []markdownParseNode{}

	i := md.parseIndex
	for i < len(md.buf) {
		if string(md.buf[i].tok_raw) == "*" {
			i++
			j := i
			for j < len(md.buf) {
				if string(md.buf[j].tok_raw) == "*" {
					/*
						if len(indexBuffer) > 0 {
							fmt.Println("i ", i)
							fmt.Println("bufthingy ", indexBuffer[len(indexBuffer)-1].idx2)
							if i == indexBuffer[len(indexBuffer)-1].idx2+2 {
								//i++
								j++
								i = j
								continue
							}
						}
					*/
					tmp := markdownParseNode{
						idx1:     i,
						idx2:     j - 1,
						property: "ITALIC",
						depth:    0,
					}
					indexBuffer = append(indexBuffer, tmp)
					//i++
					i = j + 1
					break
				}
				j++
			}
		}
		i++
	}
	if i > md.parseIndex {
		md.parseIndex = i
	}
	md.parseIndex = 0
	return indexBuffer
}

// Literally just checks for something that doesn't fall into the bounds of another parse index buffer, and flags it as a literal. All this really serves to do is count all of the data that isn't parsed as some special format.
func literalsParseRule(md *MarkdownState) []markdownParseNode {
	indexBuffer := []markdownParseNode{}
	//fmt.Println("NODDESSSS ", md.nodes)
	for i := range md.buf {
		for j := range md.nodes {
			if i < md.nodes[j].idx1 && i > md.nodes[j].idx2 {
				tmp := markdownParseNode{
					idx1:     i,
					idx2:     i + 1,
					property: "LITERAL",
					depth:    0,
				}
				indexBuffer = append(indexBuffer, tmp)
			}
		}
	}
	fmt.Println("literals index buffer ", indexBuffer)
	md.parseIndex = 0
	return indexBuffer
}

// Iterate until you find a non-thing character and then return a single header, but then if it returns a thing character, make a new one.

/*
func boldParseRule(dat []byte) (int, int) {

}
*/

func ParseMarkdownFromState(md *MarkdownState) {

	headings := headerParseRule(md)
	for heading := range headings {
		md.nodes = append(md.nodes, headings[heading])
	}

	italics := italicParseRule(md)
	for italic := range italics {
		/*
			fmt.Println("---------------------------------------------------------------")
			fmt.Println("ITALICS PARSER ", italics[italic])
			fmt.Println(md.buf[italics[italic].idx1:italics[italic].idx2])
			fmt.Println("---------------------------------------------------------------")
		*/
		md.nodes = append(md.nodes, italics[italic])
	}

	if len(md.nodes) == 0 {
		/*outputParserError(errno int, filepath string, row int, col int)*/
		outputParserError(600, md.filepath, md.row, md.col)
	}

	literals := literalsParseRule(md)
	//fmt.Println("Literals ", literals)
	for literal := range literals {
		md.nodes = append(md.nodes, literals[literal])
	}

	//fmt.Println(md.nodes)
}

// deal with the indexes in order.
