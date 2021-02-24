const serverURL = "http://localhost:8080"

const fetchState = () => {
  fetch(serverURL + '/state')
  .then(response => response.json())
  .then(data => console.log(data));
}

document.addEventListener("DOMContentLoaded", function() {
  fetchState();
});