package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// TitleMaxLength es el número máximo de caracteres permitido en el título.
const TitleMaxLength = 200

// DueDateLayout es el formato del día límite que envía y espera <input type="date">.
const DueDateLayout = "2006-01-02"

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
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

// SetDueDate asigna la fecha límite a partir del valor del formulario (aaaa-mm-dd).
// Un valor vacío quita la fecha. Si el valor no es un día válido devuelve el error
// de validación y no cambia la fecha de la tarea.
func (t *Task) SetDueDate(value string) *validate.Errors {
	verrs := validate.NewErrors()

	value = strings.TrimSpace(value)
	if value == "" {
		t.DueDate = nil
		return verrs
	}

	day, err := time.Parse(DueDateLayout, value)
	if err != nil {
		verrs.Add("due_date", "La fecha límite no es válida. Usa el formato aaaa-mm-dd.")
		return verrs
	}

	t.DueDate = &day
	return verrs
}

// HasDueDate indica si la tarea tiene fecha límite.
func (t Task) HasDueDate() bool {
	return t.DueDate != nil
}

// DueDateValue devuelve la fecha límite en formato aaaa-mm-dd, o "" si no tiene.
func (t Task) DueDateValue() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite en formato dd/mm/aaaa, o "" si no tiene.
func (t Task) DueDateLabel() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format("02/01/2006")
}
