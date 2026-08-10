package rag

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExtractTextFromFilePlainText(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := ExtractTextFromFile(path)
	if err != nil {
		t.Fatalf("ExtractTextFromFile returned error: %v", err)
	}
	if got != "hello\nworld" {
		t.Fatalf("unexpected text: %q", got)
	}
}

func TestExtractTextFromFileCSV(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "table.csv")
	if err := os.WriteFile(path, []byte("name,role\nana,dev\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := ExtractTextFromFile(path)
	if err != nil {
		t.Fatalf("ExtractTextFromFile returned error: %v", err)
	}
	if !strings.Contains(got, "name\trole") || !strings.Contains(got, "ana\tdev") {
		t.Fatalf("unexpected csv extraction: %q", got)
	}
}

func TestExtractTextFromFileDOCX(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "doc.docx")
	if err := writeMinimalDOCX(path, "First paragraph", "Second line"); err != nil {
		t.Fatalf("write docx: %v", err)
	}

	got, err := ExtractTextFromFile(path)
	if err != nil {
		t.Fatalf("ExtractTextFromFile returned error: %v", err)
	}
	if !strings.Contains(got, "First paragraph") || !strings.Contains(got, "Second line") {
		t.Fatalf("unexpected docx extraction: %q", got)
	}
}

func TestExtractTextFromFileXLSX(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sheet.xlsx")

	f := excelize.NewFile()
	defer f.Close()
	f.SetCellValue("Sheet1", "A1", "name")
	f.SetCellValue("Sheet1", "B1", "score")
	f.SetCellValue("Sheet1", "A2", "ana")
	f.SetCellValue("Sheet1", "B2", 99)
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}

	got, err := ExtractTextFromFile(path)
	if err != nil {
		t.Fatalf("ExtractTextFromFile returned error: %v", err)
	}
	if !strings.Contains(got, "# sheet: Sheet1") {
		t.Fatalf("expected sheet header in extraction, got: %q", got)
	}
	if !strings.Contains(got, "name\tscore") || !strings.Contains(got, "ana\t99") {
		t.Fatalf("unexpected xlsx extraction: %q", got)
	}
}

func TestExtractTextFromFileUnsupportedBinary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(path, []byte{0x00, 0xff, 0x10, 0x7f}, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := ExtractTextFromFile(path); err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func writeMinimalDOCX(path string, paragraphs ...string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	contentTypes, err := zw.Create("[Content_Types].xml")
	if err != nil {
		return err
	}
	if _, err := contentTypes.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`)); err != nil {
		return err
	}

	rels, err := zw.Create("_rels/.rels")
	if err != nil {
		return err
	}
	if _, err := rels.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`)); err != nil {
		return err
	}

	doc, err := zw.Create("word/document.xml")
	if err != nil {
		return err
	}
	b := strings.Builder{}
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, p := range paragraphs {
		b.WriteString(`<w:p><w:r><w:t>`)
		b.WriteString(xmlEscape(p))
		b.WriteString(`</w:t></w:r></w:p>`)
	}
	b.WriteString(`</w:body></w:document>`)
	if _, err := doc.Write([]byte(b.String())); err != nil {
		return err
	}

	return nil
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
