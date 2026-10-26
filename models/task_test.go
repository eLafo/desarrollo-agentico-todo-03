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

func (ms *ModelSuite) Test_ParseDueDate_Empty() {
	for _, value := range []string{"", "   "} {
		dueDate, err := ParseDueDate(value)
		ms.NoError(err)
		ms.False(dueDate.Valid)
	}
}

func (ms *ModelSuite) Test_ParseDueDate_Valid() {
	dueDate, err := ParseDueDate("2026-11-03")
	ms.NoError(err)
	ms.True(dueDate.Valid)
	ms.Equal(2026, dueDate.Time.Year())
	ms.Equal(time.November, dueDate.Time.Month())
	ms.Equal(3, dueDate.Time.Day())
	ms.Zero(dueDate.Time.Hour())
	ms.Zero(dueDate.Time.Minute())
	ms.Zero(dueDate.Time.Second())
}

func (ms *ModelSuite) Test_ParseDueDate_Invalid() {
	for _, value := range []string{"2026-02-30", "03/11/2026", "2026-11-03T10:00", "mañana"} {
		_, err := ParseDueDate(value)
		ms.Error(err, value)
	}
}

func (ms *ModelSuite) Test_Task_DueDate_Optional() {
	task := &Task{Title: "Sin fecha límite"}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.False(reloaded.DueDate.Valid)
	ms.Equal("", reloaded.DueDateValue())
}

func (ms *ModelSuite) Test_Task_DueDate_Persists() {
	dueDate, err := ParseDueDate("2026-11-03")
	ms.NoError(err)

	task := &Task{Title: "Con fecha límite", DueDate: dueDate}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.True(reloaded.DueDate.Valid)
	ms.Equal("2026-11-03", reloaded.DueDateValue())
}

func (ms *ModelSuite) Test_Task_DueDateLabel() {
	november, err := ParseDueDate("2026-11-03")
	ms.NoError(err)
	ms.Equal("3 de noviembre de 2026", Task{DueDate: november}.DueDateLabel())

	january, err := ParseDueDate("2027-01-15")
	ms.NoError(err)
	ms.Equal("15 de enero de 2027", Task{DueDate: january}.DueDateLabel())

	ms.Equal("", Task{}.DueDateLabel())
}
