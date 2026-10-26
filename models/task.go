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

// DueDateLayout es el formato de día (AAAA-MM-DD) de los campos de fecha HTML.
const DueDateLayout = "2006-01-02"

// dueDateLabelLayout es el formato de día con el que se muestra la fecha límite.
const dueDateLabelLayout = "02/01/2006"

// DueDateInvalidMessage es el error de validación de una fecha límite que no es un día válido.
const DueDateInvalidMessage = "La fecha límite debe ser un día válido."

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" form:"-"`

	// dueDateInvalid indica que SetDueDate recibió un valor que no es un día válido.
	dueDateInvalid bool `db:"-"`
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

	if t.dueDateInvalid {
		verrs.Add("due_date", DueDateInvalidMessage)
	}

	return verrs, nil
}

// SetDueDate asigna la fecha límite a partir de un día con formato AAAA-MM-DD, sin hora.
// Un valor vacío quita la fecha límite. Si el valor no es un día válido, la fecha
// límite no cambia y Validate informa del error.
func (t *Task) SetDueDate(value string) {
	value = strings.TrimSpace(value)
	t.dueDateInvalid = false

	if value == "" {
		t.DueDate = nil
		return
	}

	day, err := time.Parse(DueDateLayout, value)
	if err != nil {
		t.dueDateInvalid = true
		return
	}
	t.DueDate = &day
}

// HasDueDate indica si la tarea tiene fecha límite.
func (t Task) HasDueDate() bool {
	return t.DueDate != nil
}

// DueDateValue devuelve la fecha límite como AAAA-MM-DD, el formato de los campos de fecha, o "" si no tiene.
func (t Task) DueDateValue() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite como DD/MM/AAAA para mostrarla, o "" si no tiene.
func (t Task) DueDateLabel() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(dueDateLabelLayout)
}
