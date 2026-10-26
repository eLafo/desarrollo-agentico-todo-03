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

// DueDateLayout es el formato AAAA-MM-DD de <input type="date"> y del atributo datetime.
const DueDateLayout = "2006-01-02"

// DueDateDisplayLayout es el formato DD/MM/AAAA con el que se muestra la fecha límite.
const DueDateDisplayLayout = "02/01/2006"

// DueDateInvalidMessage es el error de validación de una fecha límite que no es un día válido.
const DueDateInvalidMessage = "La fecha límite no es válida."

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" form:"-"`

	// dueDateInvalid indica que SetDueDate recibió un valor que no es un día AAAA-MM-DD.
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

// SetDueDate interpreta el valor del formulario como fecha límite. Un valor vacío quita la fecha;
// uno que no sea un día AAAA-MM-DD la quita y hace que Validate informe del error.
func (t *Task) SetDueDate(value string) {
	value = strings.TrimSpace(value)
	t.DueDate = nil
	t.dueDateInvalid = false
	if value == "" {
		return
	}

	dueDate, err := time.Parse(DueDateLayout, value)
	if err != nil {
		t.dueDateInvalid = true
		return
	}
	t.DueDate = &dueDate
}

// DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si la tarea no tiene.
func (t *Task) DueDateValue() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o "" si la tarea no tiene.
func (t *Task) DueDateLabel() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateDisplayLayout)
}
