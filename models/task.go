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

// DueDateLayout es el formato de la fecha límite: solo el día, sin hora.
const DueDateLayout = "2006-01-02"

// DueDateInvalidMessage es el error que se muestra cuando la fecha límite no es un día válido.
const DueDateInvalidMessage = "La fecha límite no es válida."

var monthNames = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio",
	"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

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

// ParseDueDate interpreta una fecha límite en formato AAAA-MM-DD.
// Una cadena vacía significa que la tarea no tiene fecha límite.
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

// DueDateValue devuelve la fecha límite en formato AAAA-MM-DD, o "" si no tiene.
func (t Task) DueDateValue() string {
	if !t.DueDate.Valid {
		return ""
	}
	return t.DueDate.Time.Format(DueDateLayout)
}

// DueDateLabel devuelve la fecha límite en español, p. ej. "3 de noviembre de 2026", o "" si no tiene.
func (t Task) DueDateLabel() string {
	if !t.DueDate.Valid {
		return ""
	}
	d := t.DueDate.Time
	return fmt.Sprintf("%d de %s de %d", d.Day(), monthNames[d.Month()-1], d.Year())
}
