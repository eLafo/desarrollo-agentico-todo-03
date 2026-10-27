package models

import (
	"strings"
	"time"
)

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

func (ms *ModelSuite) Test_Task_SetDueDate_Valid() {
	task := &Task{Title: "Pagar el alquiler"}

	verrs := task.SetDueDate("2026-11-15")
	ms.False(verrs.HasAny())

	ms.True(task.HasDueDate())
	ms.Equal(2026, task.DueDate.Year())
	ms.Equal(time.November, task.DueDate.Month())
	ms.Equal(15, task.DueDate.Day())
	ms.Equal(0, task.DueDate.Hour())
	ms.Equal("2026-11-15", task.DueDateValue())
	ms.Equal("15/11/2026", task.DueDateLabel())
}

func (ms *ModelSuite) Test_Task_SetDueDate_EmptyClears() {
	task := &Task{Title: "Pagar el alquiler"}
	ms.False(task.SetDueDate("2026-11-15").HasAny())

	verrs := task.SetDueDate("   ")
	ms.False(verrs.HasAny())

	ms.Nil(task.DueDate)
	ms.False(task.HasDueDate())
	ms.Equal("", task.DueDateValue())
	ms.Equal("", task.DueDateLabel())
}

func (ms *ModelSuite) Test_Task_SetDueDate_Invalid() {
	for _, value := range []string{"2026-02-30", "15/11/2026", "2026-11-15T10:00", "mañana"} {
		task := &Task{Title: "Pagar el alquiler"}
		ms.False(task.SetDueDate("2026-11-15").HasAny())

		verrs := task.SetDueDate(value)
		ms.True(verrs.HasAny(), value)
		ms.Contains(verrs.Get("due_date"), "La fecha límite no es válida. Usa el formato aaaa-mm-dd.", value)
		ms.Equal("2026-11-15", task.DueDateValue(), value)
	}
}

func (ms *ModelSuite) Test_Task_DueDate_Persists() {
	task := &Task{Title: "Pagar el alquiler"}
	ms.False(task.SetDueDate("2026-11-15").HasAny())
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.Equal("2026-11-15", reloaded.DueDateValue())

	withoutDate := &Task{Title: "Sin fecha límite"}
	verrs, err = ms.DB.ValidateAndCreate(withoutDate)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded = &Task{}
	ms.NoError(ms.DB.Find(reloaded, withoutDate.ID))
	ms.Nil(reloaded.DueDate)
}
