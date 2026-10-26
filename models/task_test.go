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
	task := &Task{Title: "Sin fecha"}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.Nil(reloaded.DueDate)
	ms.False(reloaded.HasDueDate())
}

func (ms *ModelSuite) Test_Task_SetDueDate_PersistsDay() {
	task := &Task{Title: "Con fecha"}
	task.SetDueDate("2026-11-15")
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.True(reloaded.HasDueDate())
	ms.Equal("2026-11-15", reloaded.DueDateValue())
	ms.Equal("15/11/2026", reloaded.DueDateLabel())
	ms.Zero(reloaded.DueDate.Hour())
	ms.Zero(reloaded.DueDate.Minute())
	ms.Zero(reloaded.DueDate.Second())
}

func (ms *ModelSuite) Test_Task_SetDueDate_EmptyClears() {
	task := &Task{Title: "Con fecha"}
	task.SetDueDate("2026-11-15")
	ms.True(task.HasDueDate())

	task.SetDueDate("")
	ms.Nil(task.DueDate)

	verrs, err := task.Validate(ms.DB)
	ms.NoError(err)
	ms.False(verrs.HasAny())
}

func (ms *ModelSuite) Test_Task_SetDueDate_Invalid() {
	for _, value := range []string{"15/11/2026", "2026-13-40", "2026-11-15T10:00", "mañana"} {
		task := &Task{Title: "Fecha inválida"}
		task.SetDueDate(value)

		verrs, err := ms.DB.ValidateAndCreate(task)
		ms.NoError(err)
		ms.True(verrs.HasAny(), value)
		ms.Contains(verrs.Get("due_date"), "La fecha límite debe ser un día válido.", value)
	}

	count, err := ms.DB.Count(&Task{})
	ms.NoError(err)
	ms.Equal(0, count)
}
