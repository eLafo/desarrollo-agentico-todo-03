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

// DueDateLayout es el formato de día (sin hora) de la fecha límite, el mismo
// que usan <input type="date"> y el atributo datetime.
const DueDateLayout = "2006-01-02"

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" form:"-"`

	// dueDateInvalid indica que SetDueDate recibió un valor que no es un día AAAA-MM-DD.
	dueDateInvalid bool `json:"-" db:"-" form:"-"`
}

// Tasks es una colección de Task.
type Tasks []Task

// SetDueDate asigna la fecha límite a partir de un día AAAA-MM-DD. Un valor
// vacío la quita; uno inválido la deja sin cambios y Validate lo rechaza.
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

// DueDateValue devuelve la fecha límite como AAAA-MM-DD, o "" si no hay.
func (t Task) DueDateValue() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite como DD/MM/AAAA, o "" si no hay.
func (t Task) DueDateLabel() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format("02/01/2006")
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

	if t.dueDateInvalid {
		verrs.Add("due_date", "La fecha límite debe ser un día válido.")
	}

	return verrs, nil
}
