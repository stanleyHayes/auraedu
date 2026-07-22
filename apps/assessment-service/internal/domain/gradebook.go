package domain

import (
	"math"
	"sort"
)

// GradeRow is one recorded score joined with its assessment's subject and
// max_score — the input to gradebook aggregation.
type GradeRow struct {
	SubjectID string
	Score     int
	MaxScore  int
}

// GradeAggregate summarizes scores over a set of assessments. Average is the
// arithmetic mean of per-assessment percentages; WeightedAverage weights each
// assessment by its max_score (total_score / total_max_score * 100). Both are
// percentages in [0, 100], nil when there are no scores.
type GradeAggregate struct {
	AssessmentCount int      `json:"assessment_count"`
	TotalScore      int      `json:"total_score"`
	TotalMaxScore   int      `json:"total_max_score"`
	Average         *float64 `json:"average"`
	WeightedAverage *float64 `json:"weighted_average"`
}

// SubjectGrade is a GradeAggregate for one subject.
type SubjectGrade struct {
	SubjectID string `json:"subject_id"`
	GradeAggregate
}

// Gradebook is a per-student or per-class gradebook summary: per-subject
// aggregates plus an overall aggregate across all subjects.
type Gradebook struct {
	StudentID      string         `json:"student_id,omitempty"`
	ClassID        string         `json:"class_id,omitempty"`
	AcademicYearID string         `json:"academic_year_id,omitempty"`
	Subjects       []SubjectGrade `json:"subjects"`
	Overall        GradeAggregate `json:"overall"`
}

// AggregateGrades computes per-subject and overall grade aggregates from raw
// score rows. Rows with a non-positive max_score are skipped (they cannot
// contribute a percentage). Subjects are ordered by subject_id for
// deterministic output.
func AggregateGrades(rows []GradeRow) Gradebook {
	bySubject := make(map[string][]GradeRow)
	for _, r := range rows {
		if r.MaxScore <= 0 {
			continue
		}
		bySubject[r.SubjectID] = append(bySubject[r.SubjectID], r)
	}

	subjectIDs := make([]string, 0, len(bySubject))
	for id := range bySubject {
		subjectIDs = append(subjectIDs, id)
	}
	sort.Strings(subjectIDs)

	book := Gradebook{Subjects: make([]SubjectGrade, 0, len(subjectIDs))}
	for _, id := range subjectIDs {
		book.Subjects = append(book.Subjects, SubjectGrade{SubjectID: id, GradeAggregate: aggregate(bySubject[id])})
	}

	all := make([]GradeRow, 0, len(rows))
	for _, r := range rows {
		if r.MaxScore <= 0 {
			continue
		}
		all = append(all, r)
	}
	book.Overall = aggregate(all)
	return book
}

func aggregate(rows []GradeRow) GradeAggregate {
	agg := GradeAggregate{}
	if len(rows) == 0 {
		return agg
	}
	var pctSum float64
	for _, r := range rows {
		agg.AssessmentCount++
		agg.TotalScore += r.Score
		agg.TotalMaxScore += r.MaxScore
		pctSum += float64(r.Score) / float64(r.MaxScore) * 100
	}
	avg := round2(pctSum / float64(len(rows)))
	weighted := round2(float64(agg.TotalScore) / float64(agg.TotalMaxScore) * 100)
	agg.Average = &avg
	agg.WeightedAverage = &weighted
	return agg
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
