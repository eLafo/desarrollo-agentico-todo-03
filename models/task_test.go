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

func (ms *ModelSuite) Test_ParseDueDate() {
	for _, value := range []string{"", "   "} {
		dueDate, ok := ParseDueDate(value)
		ms.True(ok, value)
		ms.Nil(dueDate, value)
	}

	dueDate, ok := ParseDueDate("2026-11-15")
	ms.True(ok)
	ms.NotNil(dueDate)
	ms.Equal(2026, dueDate.Year())
	ms.Equal(time.November, dueDate.Month())
	ms.Equal(15, dueDate.Day())

	for _, value := range []string{"2026-02-30", "15/11/2026", "2026-11-15T10:00", "mañana"} {
		dueDate, ok := ParseDueDate(value)
		ms.False(ok, value)
		ms.Nil(dueDate, value)
	}
}

func (ms *ModelSuite) Test_Task_DueDate_Optional() {
	task := &Task{Title: "Sin fecha límite"}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.Nil(reloaded.DueDate)
	ms.Equal("", reloaded.DueDateValue())
	ms.Equal("", reloaded.DueDateLabel())
}

func (ms *ModelSuite) Test_Task_DueDate_Persists() {
	dueDate, ok := ParseDueDate("2026-11-15")
	ms.True(ok)

	task := &Task{Title: "Con fecha límite", DueDate: dueDate}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.Equal("2026-11-15", reloaded.DueDateValue())
	ms.Equal("15/11/2026", reloaded.DueDateLabel())
}
