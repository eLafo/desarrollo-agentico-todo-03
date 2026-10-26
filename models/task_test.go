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

func (ms *ModelSuite) Test_Task_DueDate_OptionalByDefault() {
	task := &Task{Title: "Sin fecha"}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.False(reloaded.DueDate.Valid)
}

func (ms *ModelSuite) Test_Task_DueDate_PersistsDay() {
	task := &Task{
		Title:   "Pagar el alquiler",
		DueDate: nulls.NewTime(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)),
	}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.True(reloaded.DueDate.Valid)
	ms.Equal("2026-10-30", reloaded.DueDate.Time.Format(DueDateLayout))
}

func (ms *ModelSuite) Test_ParseDueDate() {
	for _, value := range []string{"", "   "} {
		dueDate, err := ParseDueDate(value)
		ms.NoError(err)
		ms.False(dueDate.Valid)
	}

	dueDate, err := ParseDueDate("2026-10-30")
	ms.NoError(err)
	ms.True(dueDate.Valid)
	ms.Equal("2026-10-30", dueDate.Time.Format(DueDateLayout))

	for _, value := range []string{"30/10/2026", "2026-02-30", "2026-10-30T10:00", "mañana"} {
		_, err := ParseDueDate(value)
		ms.Error(err, value)
	}
}

func (ms *ModelSuite) Test_Task_DueDateFormatting() {
	task := Task{DueDate: nulls.NewTime(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC))}
	ms.Equal("2026-10-30", task.DueDateISO())
	ms.Equal("30/10/2026", task.DueDateDisplay())

	task = Task{}
	ms.Equal("", task.DueDateISO())
	ms.Equal("", task.DueDateDisplay())
}
