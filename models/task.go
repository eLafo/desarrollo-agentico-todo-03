package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// TitleMaxLength es el número máximo de caracteres permitido en el título.
const TitleMaxLength = 200

// DueDateLayout es el formato de día, sin hora, con el que se recibe la fecha límite en formularios.
const DueDateLayout = "2006-01-02"

// ErrInvalidDueDate indica que la fecha límite recibida no es un día válido en formato DueDateLayout.
var ErrInvalidDueDate = errors.New("La fecha límite debe ser un día válido.")

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID  `json:"id" db:"id" form:"-"`
	Title     string     `json:"title" db:"title" form:"title"`
	Completed bool       `json:"completed" db:"completed" form:"-"`
	DueDate   *time.Time `json:"due_date" db:"due_date" form:"-"`
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

// ParseDueDate convierte un día en formato AAAA-MM-DD en fecha límite.
// Un valor vacío o con solo espacios significa que la tarea no tiene fecha límite.
func ParseDueDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	dueDate, err := time.Parse(DueDateLayout, value)
	if err != nil {
		return nil, ErrInvalidDueDate
	}
	return &dueDate, nil
}

// HasDueDate indica si la tarea tiene fecha límite.
func (t Task) HasDueDate() bool {
	return t.DueDate != nil
}

// DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o una cadena vacía si no tiene.
func (t Task) DueDateValue() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite en formato DD/MM/AAAA, o una cadena vacía si no tiene.
func (t Task) DueDateLabel() string {
	if t.DueDate == nil {
		return ""
	}
	return t.DueDate.Format("02/01/2006")
}
