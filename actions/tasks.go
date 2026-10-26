package actions

import (
	"net/http"
	"strings"

	"todo/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
)

// taskInput recoge los campos de una tarea que el usuario puede enviar desde los formularios.
type taskInput struct {
	Title   string `form:"title" json:"title"`
	DueDate string `form:"due_date" json:"due_date"`
}

// applyTo asigna el título y la fecha límite a la tarea. Si la fecha no es válida,
// devuelve los errores de validación de la tarea junto con el de la fecha, sin guardar nada.
func (in taskInput) applyTo(tx *pop.Connection, task *models.Task) (*validate.Errors, error) {
	task.Title = strings.TrimSpace(in.Title)

	dueDate, err := models.ParseDueDate(in.DueDate)
	if err == nil {
		task.DueDate = dueDate
		return validate.NewErrors(), nil
	}

	verrs, err := task.Validate(tx)
	if err != nil {
		return nil, err
	}
	verrs.Add("due_date", models.DueDateInvalidMessage)
	return verrs, nil
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
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/index.plush.html"))
}

// TasksCreate crea una tarea nueva a partir del formulario.
func TasksCreate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	input := taskInput{}
	if err := c.Bind(&input); err != nil {
		return err
	}

	task := &models.Task{}
	verrs, err := input.applyTo(tx, task)
	if err != nil {
		return err
	}
	if !verrs.HasAny() {
		if verrs, err = tx.ValidateAndCreate(task); err != nil {
			return err
		}
	}

	if verrs.HasAny() {
		tasks := models.Tasks{}
		if err := tx.Order("created_at asc, id asc").All(&tasks); err != nil {
			return err
		}

		c.Set("tasks", tasks)
		c.Set("task", task)
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
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/edit.plush.html"))
}

// TasksUpdate actualiza el título y la fecha límite, conservando los demás datos de la tarea.
func TasksUpdate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	input := taskInput{}
	if err := c.Bind(&input); err != nil {
		return err
	}

	verrs, err := input.applyTo(tx, task)
	if err != nil {
		return err
	}
	if !verrs.HasAny() {
		if verrs, err = tx.ValidateAndUpdate(task); err != nil {
			return err
		}
	}
	if verrs.HasAny() {
		c.Set("task", task)
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
