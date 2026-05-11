package exporter

import (
	"encoding/csv"
	"os"
	"sort"
	"strings"
	"time"

	"ghclassroom/internal/api"
	"github.com/xuri/excelize/v2"
)

type CommitRow struct {
	AssignmentTitle string
	StudentLogin    string
	RepoFullName    string
	SHA             string
	Date            string
	Message         string
	Branch          string
}

// BuildRows assembles one row per commit across all students, sorted by
// StudentLogin ASC then Date ASC.
func BuildRows(assignmentTitle string, students []api.AcceptedAssignment, activities map[string]*api.RepoActivity) []CommitRow {
	var rows []CommitRow
	for _, s := range students {
		if len(s.Students) == 0 {
			continue
		}
		login := s.Students[0].Login
		act, ok := activities[s.Repository.FullName]
		if !ok || act == nil {
			continue
		}
		for _, c := range act.Commits {
			msg := c.Commit.Message
			if nl := strings.IndexByte(msg, '\n'); nl >= 0 {
				msg = msg[:nl]
			}
			if len([]rune(msg)) > 120 {
				msg = string([]rune(msg)[:120])
			}
			date := c.Commit.Author.Date
			if t, err := time.Parse(time.RFC3339, date); err == nil {
				date = t.UTC().Format("02-01-2006 15:04 UTC")
			}
			rows = append(rows, CommitRow{
				AssignmentTitle: assignmentTitle,
				StudentLogin:    login,
				RepoFullName:    s.Repository.FullName,
				SHA:             c.SHA,
				Date:            date,
				Message:         msg,
				Branch:          "N/A",
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StudentLogin != rows[j].StudentLogin {
			return rows[i].StudentLogin < rows[j].StudentLogin
		}
		return rows[i].Date < rows[j].Date
	})
	return rows
}

func ExportCSV(rows []CommitRow, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"Assignment", "Student", "Repository", "SHA", "Date", "Message"}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write([]string{
			r.AssignmentTitle, r.StudentLogin, r.RepoFullName, r.SHA, r.Date, r.Message,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func ExportExcel(rows []CommitRow, outputPath string) error {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Commit History"
	f.SetSheetName("Sheet1", sheet)

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
	})
	if err != nil {
		return err
	}

	headers := []string{"Assignment", "Student", "Repository", "SHA", "Date", "Message"}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	if err := f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return err
	}

	for i, r := range rows {
		rowNum := i + 2
		vals := []string{r.AssignmentTitle, r.StudentLogin, r.RepoFullName, r.SHA, r.Date, r.Message}
		for col, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
			f.SetCellValue(sheet, cell, v)
		}
	}

	colWidths := []struct {
		col   string
		width float64
	}{
		{"A", 30}, {"B", 20}, {"C", 35}, {"D", 12}, {"E", 22}, {"F", 60},
	}
	for _, cw := range colWidths {
		if err := f.SetColWidth(sheet, cw.col, cw.col, cw.width); err != nil {
			return err
		}
	}

	return f.SaveAs(outputPath)
}
