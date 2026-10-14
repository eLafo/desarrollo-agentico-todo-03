// Refleja en la tarea el estado completada o pendiente.
function setTaskCompleted(task, form, completed) {
  task.classList.toggle("task-completed", completed);
  var button = form.querySelector("button");
  var status = task.querySelector(".task-status");

  var label = completed ? "Reabrir" : "Completar";
  button.querySelector(".toggle-label").textContent = label;
  button.setAttribute("aria-pressed", String(completed));
  button.setAttribute("aria-label", label + " tarea: " + task.querySelector(".task-title").textContent);
  button.setAttribute("title", label + " tarea");

  if (completed && !status) {
    status = document.createElement("span");
    status.className = "task-status";
    status.textContent = "Completada";
    task.insertBefore(status, task.querySelector(".task-actions"));
  } else if (!completed && status) {
    status.remove();
  }
}

// Pide confirmación antes de enviar formularios marcados con data-confirm.
document.addEventListener("submit", function (event) {
  var form = event.target;
  if (form.dataset.confirm && !window.confirm(form.dataset.confirm)) {
    event.preventDefault();
  }

  if (!form.matches("[data-task-toggle]")) {
    return;
  }

  event.preventDefault();

  var task = form.closest(".task");
  var completed = !task.classList.contains("task-completed");
  setTaskCompleted(task, form, completed);

  // Envía los campos del formulario (_method y authenticity_token) como el envío nativo.
  fetch(form.action, {
    method: "POST",
    body: new URLSearchParams(new FormData(form)),
    credentials: "same-origin"
  })
    .then(function (response) {
      if (!response.ok) {
        throw new Error("No se pudo guardar el estado de la tarea");
      }
    })
    .catch(function () {
      setTaskCompleted(task, form, !completed);
    });
});
