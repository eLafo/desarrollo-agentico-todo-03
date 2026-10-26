package models

import "strings"

func (ms *ModelSuite) Test_Task_Create_Valid() {
	task := &Task{Title: "Comprar café"}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	ms.NotZero(task.ID)
	ms.False(task.Completed)

	count, err := ms.DB.Count(&Task{})
	ms.NoError(err)
	ms.Equal(1, count)
}

func (ms *ModelSuite) Test_Task_Title_Required() {
	task := &Task{Title: ""}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.True(verrs.HasAny())
	ms.Contains(verrs.Get("title"), "El título es obligatorio.")

	count, err := ms.DB.Count(&Task{})
	ms.NoError(err)
	ms.Equal(0, count)
}

func (ms *ModelSuite) Test_Task_Title_OnlySpaces_Invalid() {
	task := &Task{Title: "   \t  "}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.True(verrs.HasAny())
	ms.Contains(verrs.Get("title"), "El título es obligatorio.")
}

func (ms *ModelSuite) Test_Task_Title_MaxLength() {
	task := &Task{Title: strings.Repeat("a", TitleMaxLength)}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())
}

func (ms *ModelSuite) Test_Task_Title_TooLong_Invalid() {
	task := &Task{Title: strings.Repeat("a", TitleMaxLength+1)}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.True(verrs.HasAny())
	ms.NotEmpty(verrs.Get("title"))
}

func (ms *ModelSuite) Test_Task_Toggle_Persists() {
	task := &Task{Title: "Revisar el correo"}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	task.Completed = true
	ms.NoError(ms.DB.Update(task))

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.True(reloaded.Completed)
}

func (ms *ModelSuite) Test_Task_DueDate_OptionalByDefault() {
	task := &Task{Title: "Sin fecha límite"}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.False(reloaded.DueDate.Valid)
	ms.Equal("", reloaded.DueDateValue())
	ms.Equal("", reloaded.DueDateLabel())
}

func (ms *ModelSuite) Test_Task_DueDate_Persists() {
	task := &Task{Title: "Entregar el informe"}
	task.SetDueDate("2026-11-15")
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.True(reloaded.DueDate.Valid)
	ms.Equal("2026-11-15", reloaded.DueDateValue())
	ms.Equal("15/11/2026", reloaded.DueDateLabel())
}

func (ms *ModelSuite) Test_Task_DueDate_EmptyClears() {
	task := &Task{Title: "Con fecha límite"}
	task.SetDueDate("2026-11-15")
	ms.True(task.DueDate.Valid)

	task.SetDueDate("   ")
	ms.False(task.DueDate.Valid)

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())
}

func (ms *ModelSuite) Test_Task_DueDate_Invalid() {
	for _, value := range []string{"2026-02-30", "15/11/2026", "2026-11-15T10:00", "mañana", "0000-01-01"} {
		task := &Task{Title: "Fecha no válida"}
		task.SetDueDate(value)

		verrs, err := ms.DB.ValidateAndCreate(task)
		ms.NoError(err)
		ms.True(verrs.HasAny(), value)
		ms.Contains(verrs.Get("due_date"), "La fecha límite no es un día válido.", value)

		count, err := ms.DB.Count(&Task{})
		ms.NoError(err)
		ms.Equal(0, count, value)
	}
}

func (ms *ModelSuite) Test_Task_DueDate_ValidAfterInvalid() {
	task := &Task{Title: "Corregir la fecha"}
	task.SetDueDate("2026-02-30")
	task.SetDueDate("2026-11-15")

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())
	ms.Equal("2026-11-15", task.DueDateValue())
}
