package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// TitleMaxLength es el número máximo de caracteres permitido en el título.
const TitleMaxLength = 200

// DueDateLayout es el formato de la fecha límite en formularios y atributos datetime.
const DueDateLayout = "2006-01-02"

// DueDateDisplayLayout es el formato visible de la fecha límite.
const DueDateDisplayLayout = "02/01/2006"

// Task representa una tarea de la lista.
type Task struct {
	ID             uuid.UUID  `json:"id" db:"id" form:"-"`
	Title          string     `json:"title" db:"title" form:"title"`
	Completed      bool       `json:"completed" db:"completed" form:"-"`
	DueDate        nulls.Time `json:"due_date" db:"due_date" form:"-"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at" form:"-"`
	invalidDueDate bool       `json:"-" db:"-" form:"-"`
}

// Tasks es una colección de Task.
type Tasks []Task

// SetDueDate asigna la fecha límite a partir de un valor AAAA-MM-DD; un valor vacío la quita.
func (t *Task) SetDueDate(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		t.DueDate = nulls.Time{}
		t.invalidDueDate = false
		return
	}

	d, err := time.Parse(DueDateLayout, value)
	if err != nil {
		t.DueDate = nulls.Time{}
		t.invalidDueDate = true
		return
	}

	t.DueDate = nulls.NewTime(d)
	t.invalidDueDate = false
}

// DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si no tiene.
func (t Task) DueDateValue() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o "" si no tiene.
func (t Task) DueDateLabel() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateDisplayLayout)
}

// Validate se ejecuta en cada llamada a los métodos "pop.Validate*".
func (t *Task) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.NewErrors()

	title := strings.TrimSpace(t.Title)
	if title == "" {
		verrs.Add("title", "El título es obligatorio.")
	} else if len([]rune(title)) > TitleMaxLength {
		verrs.Add("title", fmt.Sprintf("El título no puede superar los %d caracteres.", TitleMaxLength))
	}

	if t.invalidDueDate {
		verrs.Add("due_date", "La fecha límite no es válida.")
	}

	return verrs, nil
}
