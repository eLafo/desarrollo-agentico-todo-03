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

// DueDateLayout es el formato de la fecha límite en formularios: solo el día, sin hora.
const DueDateLayout = "2006-01-02"

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

// ParseDueDate interpreta una fecha límite con formato AAAA-MM-DD.
// Una cadena vacía significa que la tarea no tiene fecha límite.
func ParseDueDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	d, err := time.Parse(DueDateLayout, value)
	if err != nil {
		return nil, err
	}
	if d.Year() < 1 {
		return nil, fmt.Errorf("año fuera de rango: %d", d.Year())
	}
	return &d, nil
}

// DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si no tiene.
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
