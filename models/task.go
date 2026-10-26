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

// DueDateLayout es el formato AAAA-MM-DD con el que se recibe la fecha límite.
const DueDateLayout = "2006-01-02"

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	DueDate   nulls.Time `json:"due_date" db:"due_date" form:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" form:"-"`
}

// Tasks es una colección de Task.
type Tasks []Task

// Validate se ejecuta en cada llamada a los métodos "pop.Validate*".
func (t *Task) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.NewErrors()

	title := strings.TrimSpace(t.Title)
	if title == "" {
		verrs.Add("title", "El título es obligatorio.")
	} else if len([]rune(title)) > TitleMaxLength {
		verrs.Add("title", fmt.Sprintf("El título no puede superar los %d caracteres.", TitleMaxLength))
	}

	return verrs, nil
}

// ParseDueDate convierte el valor de un campo de fecha (AAAA-MM-DD) en una
// fecha límite opcional. Un valor vacío significa que la tarea no tiene fecha límite.
func ParseDueDate(value string) (nulls.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nulls.Time{}, nil
	}
	day, err := time.Parse(DueDateLayout, value)
	if err != nil {
		return nulls.Time{}, err
	}
	return nulls.NewTime(day), nil
}

// DueDateISO devuelve la fecha límite como AAAA-MM-DD, o "" si no tiene.
func (t Task) DueDateISO() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateLayout)
}

// DueDateDisplay devuelve la fecha límite como DD/MM/AAAA, o "" si no tiene.
func (t Task) DueDateDisplay() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format("02/01/2006")
}
