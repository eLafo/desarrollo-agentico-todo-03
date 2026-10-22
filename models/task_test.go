package models

import (
	"strings"
	"time"

	"github.com/gobuffalo/nulls"
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

func (ms *ModelSuite) Test_Task_DueDate_Persists() {
	task := &Task{Title: "Pagar impuestos", DueDate: nulls.NewTime(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	found := &Task{}
	ms.NoError(ms.DB.Find(found, task.ID))
	ms.True(found.DueDate.Valid)
	ms.Equal("2026-12-31", found.DueDateValue())
	ms.Equal("31/12/2026", found.DueDateLabel())
}

func (ms *ModelSuite) Test_Task_DueDate_Optional() {
	task := &Task{Title: "Sin fecha"}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	found := &Task{}
	ms.NoError(ms.DB.Find(found, task.ID))
	ms.False(found.DueDate.Valid)
	ms.Equal("", found.DueDateValue())
	ms.Equal("", found.DueDateLabel())
}

func (ms *ModelSuite) Test_Task_Overdue() {
	today := time.Date(2026, 10, 22, 15, 30, 0, 0, time.UTC)
	due := func(day int, completed bool) Task {
		return Task{Completed: completed, DueDate: nulls.NewTime(time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC))}
	}

	ms.True(due(21, false).Overdue(today))
	ms.False(due(22, false).Overdue(today))
	ms.False(due(23, false).Overdue(today))
	ms.False(due(21, true).Overdue(today))
	ms.False(Task{}.Overdue(today))
}
