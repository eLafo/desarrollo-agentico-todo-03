package actions

import (
	"net/http"
	"strings"
	"time"

	"todo/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
)

// dueDateInvalid es el mensaje de error de una fecha límite mal formada.
const dueDateInvalid = "La fecha límite no es válida."

// parseDueDate convierte el valor YYYY-MM-DD de un formulario en fecha límite.
// Un valor vacío es válido y equivale a no tener fecha límite.
func parseDueDate(raw string) (nulls.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nulls.Time{}, true
	}
	d, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nulls.Time{}, false
	}
	return nulls.NewTime(d), true
}

// TasksIndex muestra la lista de tareas y el formulario de creación.
func TasksIndex(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	tasks := models.Tasks{}
	if err := tx.Order("created_at asc, id asc").All(&tasks); err != nil {
		return err
	}

	c.Set("tasks", tasks)
	c.Set("task", &models.Task{})
	c.Set("today", time.Now())
	c.Set("dueDateRaw", "")
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/index.plush.html"))
}

// TasksCreate crea una tarea nueva a partir del formulario.
func TasksCreate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := c.Bind(task); err != nil {
		return err
	}
	task.Title = strings.TrimSpace(task.Title)

	rawDue := c.Param("due_date")
	dueDate, dueOK := parseDueDate(rawDue)
	task.DueDate = dueDate

	var verrs *validate.Errors
	if dueOK {
		var err error
		verrs, err = tx.ValidateAndCreate(task)
		if err != nil {
			return err
		}
	} else {
		verrs, _ = task.Validate(tx)
		verrs.Add("due_date", dueDateInvalid)
	}

	if verrs.HasAny() {
		tasks := models.Tasks{}
		if err := tx.Order("created_at asc, id asc").All(&tasks); err != nil {
			return err
		}

		c.Set("tasks", tasks)
		c.Set("task", task)
		c.Set("today", time.Now())
		c.Set("dueDateRaw", rawDue)
		c.Set("errors", verrs)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("tasks/index.plush.html"))
	}

	c.Flash().Add("success", "Tarea creada.")
	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksEdit muestra el formulario de edición de una tarea existente.
func TasksEdit(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("task", task)
	c.Set("dueDateRaw", task.DueDateValue())
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/edit.plush.html"))
}

// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.
// Si el formulario no incluye due_date se conserva la fecha; si llega vacío se elimina.
func TasksUpdate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	input := struct {
		Title string `form:"title" json:"title"`
	}{}
	if err := c.Bind(&input); err != nil {
		return err
	}
	task.Title = strings.TrimSpace(input.Title)

	rawDue := task.DueDateValue()
	dueOK := true
	if values, present := c.Request().Form["due_date"]; present && len(values) > 0 {
		rawDue = values[0]
		var dueDate nulls.Time
		if dueDate, dueOK = parseDueDate(rawDue); dueOK {
			task.DueDate = dueDate
		}
	}

	var verrs *validate.Errors
	if dueOK {
		var err error
		verrs, err = tx.ValidateAndUpdate(task)
		if err != nil {
			return err
		}
	} else {
		verrs, _ = task.Validate(tx)
		verrs.Add("due_date", dueDateInvalid)
	}
	if verrs.HasAny() {
		c.Set("task", task)
		c.Set("dueDateRaw", rawDue)
		c.Set("errors", verrs)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("tasks/edit.plush.html"))
	}

	c.Flash().Add("success", "Tarea actualizada.")
	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksToggle alterna una tarea entre completada y pendiente.
func TasksToggle(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	task.Completed = !task.Completed
	if err := tx.Update(task); err != nil {
		return err
	}

	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksDestroy elimina una tarea.
func TasksDestroy(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(task); err != nil {
		return err
	}

	c.Flash().Add("success", "Tarea eliminada.")
	return c.Redirect(http.StatusSeeOther, "/")
}
