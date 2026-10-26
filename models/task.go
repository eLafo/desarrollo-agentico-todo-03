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

// DueDateLayout es el formato de la fecha límite: solo el día, sin hora (AAAA-MM-DD).
const DueDateLayout = "2006-01-02"

// dueDateDisplayLayout es el formato con el que se muestra la fecha límite (DD/MM/AAAA).
const dueDateDisplayLayout = "02/01/2006"

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	DueDate   nulls.Time `json:"due_date" db:"due_date" form:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" form:"-"`

	// invalidDueDate indica que SetDueDate recibió un valor que no es un día válido.
	invalidDueDate bool `db:"-"`
}

// Tasks es una colección de Task.
type Tasks []Task

// SetDueDate asigna la fecha límite a partir de un día con formato AAAA-MM-DD.
// Un valor vacío quita la fecha límite; uno no válido se rechaza en Validate.
func (t *Task) SetDueDate(value string) {
	t.invalidDueDate = false

	value = strings.TrimSpace(value)
	if value == "" {
		t.DueDate = nulls.Time{}
		return
	}

	day, err := time.Parse(DueDateLayout, value)
	if err != nil || day.Year() < 1 {
		t.invalidDueDate = true
		return
	}
	t.DueDate = nulls.NewTime(day)
}

// DueDateValue devuelve la fecha límite con formato AAAA-MM-DD, o "" si no tiene.
func (t Task) DueDateValue() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite con formato DD/MM/AAAA, o "" si no tiene.
func (t Task) DueDateLabel() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(dueDateDisplayLayout)
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
		verrs.Add("due_date", "La fecha límite no es un día válido.")
	}

	return verrs, nil
}
