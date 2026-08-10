package rag

import (
	"archive/zip"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
)

var textFileExtensions = map[string]struct{}{
	".txt":      {},
	".md":       {},
	".markdown": {},
	".rst":      {},
	".json":     {},
	".yaml":     {},
	".yml":      {},
	".xml":      {},
	".html":     {},
	".htm":      {},
	".csv":      {},
	".tsv":      {},
}

func SupportedExtractionSummary() string {
	return "txt/md/json/yaml/xml/html/csv/tsv/docx/xlsx/pdf"
}

func ExtractTextFromFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return extractPDF(path)
	case ".docx":
		return extractDOCX(path)
	case ".csv":
		return extractDelimited(path, ',')
	case ".tsv":
		return extractDelimited(path, '\t')
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return extractXLSX(path)
	default:
		if _, ok := textFileExtensions[ext]; ok || ext == "" {
			return extractPlainText(path)
		}
		return "", fmt.Errorf("unsupported file format %q", ext)
	}
}

func extractPlainText(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "", nil
	}
	if !isLikelyText(raw) {
		return "", fmt.Errorf("file is not plain text")
	}
	return strings.TrimSpace(normalizeLineEndings(string(raw))), nil
}

func extractDelimited(path string, comma rune) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = comma
	r.FieldsPerRecord = -1

	rows, err := r.ReadAll()
	if err != nil {
		return "", err
	}

	b := strings.Builder{}
	for _, row := range rows {
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
		if len(strings.TrimSpace(strings.Join(row, ""))) == 0 {
			continue
		}
		b.WriteString(strings.Join(row, "\t"))
		b.WriteByte('\n')
	}

	return strings.TrimSpace(b.String()), nil
}

func extractXLSX(path string) (string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	b := strings.Builder{}
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		if len(rows) == 0 {
			continue
		}

		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("# sheet: ")
		b.WriteString(sheet)
		b.WriteByte('\n')

		for _, row := range rows {
			for i := range row {
				row[i] = strings.TrimSpace(row[i])
			}
			if len(strings.TrimSpace(strings.Join(row, ""))) == 0 {
				continue
			}
			b.WriteString(strings.Join(row, "\t"))
			b.WriteByte('\n')
		}
	}

	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", fmt.Errorf("no readable worksheet content")
	}

	return out, nil
}

func extractDOCX(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	var docXML io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docXML, err = f.Open()
			if err != nil {
				return "", err
			}
			break
		}
	}
	if docXML == nil {
		return "", fmt.Errorf("missing word/document.xml")
	}
	defer docXML.Close()

	dec := xml.NewDecoder(docXML)
	b := strings.Builder{}
	inText := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(t.Name.Local)
			switch local {
			case "t":
				inText = true
			case "tab":
				b.WriteByte('\t')
			case "br", "cr":
				b.WriteByte('\n')
			}
		case xml.EndElement:
			local := strings.ToLower(t.Name.Local)
			switch local {
			case "t":
				inText = false
			case "p":
				b.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				b.Write([]byte(t))
			}
		}
	}

	out := strings.TrimSpace(normalizeLineEndings(b.String()))
	if out == "" {
		return "", fmt.Errorf("no readable document text")
	}

	return out, nil
}

func extractPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	plain, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	raw, err := io.ReadAll(plain)
	if err != nil {
		return "", err
	}

	out := strings.TrimSpace(normalizeLineEndings(string(raw)))
	if out == "" {
		return "", fmt.Errorf("no readable pdf text")
	}

	return out, nil
}

func isLikelyText(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	for _, b := range raw {
		if b == 0 {
			return false
		}
	}
	return true
}

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}
