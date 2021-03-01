const serverURL = "http://localhost:8080"
const nikkiHaley = 1
const kamalaHarris = 2

// verifies the consistency of the election
const verifyConsistency = async () => {
  VerifyConsistency(serverURL);
}

// updates the election stats shown in the UI
const updateStats = async () => {
  fetch(serverURL + '/stats').then(r => r.json()).then(data => {
    if (data["results"][1]) {
      document.getElementById("haley-votes").innerText = data["results"][1];
    } else {
      document.getElementById("haley-votes").innerText = "0";
    }
    if (data["results"][2]) {
      document.getElementById("harris-votes").innerText = data["results"][2];
    } else {
      document.getElementById("harris-votes").innerText = "0";
    }
    document.getElementById("registered").innerText = data["registered"];
    document.getElementById("ballots").innerText = data["ballots"];
  });
}

// updates the ballot status (if ballot ID is present)
var updateBallotStatusRunning = false
const updateBallotStatus = async () => {
  if (updateBallotStatusRunning) {
    return
  }
  updateBallotStatusRunning = true;
  const ballotID = document.getElementById("ballot-id").value;
  if (ballotID == "") {
    updateBallotStatusRunning = false;
    return
  }
  let url = new URL(serverURL + '/ballot');
  url.search = new URLSearchParams({ ballot_id: ballotID, });
  fetch(url).then(r => {
    if (r.ok) {
      r.json().then(data => {
        if (!data.vote) {
          document.getElementById('ballot-status').innerText = "Not cast";
        } else {
          if (data.vote == 1) {
            document.getElementById('ballot-status').innerText = "Cast for Nikki Haley";
          } else if (data.vote == 2) {
            document.getElementById('ballot-status').innerText = "Cast for Kamala Harris";
          } else {
            document.getElementById('ballot-status').innerText = "Invalid"
            console.log("invalid ballot vote", data.vote)
          }
        }
        updateBallotStatusRunning = false;
      });
    } else {
      r.text().then(rText => {
        console.log("got not-ok response when fetching ballot", rText)
      });
      updateBallotStatusRunning = false;
    }
  }).catch(err => {
    console.log("error fetching ballot", err);
    updateBallotStatusRunning = false;
  });
}

// verifies a random vote (checks that it has not been edited after it has been cast)
var verifyRandomVoteRunning = false
const verifyRandomVote = async () => {
  if (verifyRandomVoteRunning) {
    return
  }
  verifyRandomVoteRunning = true;
  fetch(serverURL + '/random-ballot').then(r => {
    if (r.ok) {
      r.json().then(data => {
        let ok = '<span class="audit-ok">OK</span>'
        if (data.history.length > 2) {
          ok = '<span class="audit-failed">Not OK: change after cast!</span>'
        }
        ok += ' @ ' + (new Date()).toISOString();
        if (data.vote == 0) {
          data.vote = "Registered";
        } else if (data.vote == 1) {
          data.vote = "Nikki Haley";
        } else if (data.vote == 2) {
          data.vote = "Kamala Harris";
        } else {
          data.vote = "Invalid vote '"+data.history[i]+"'";
        }
        let history = [];
        for (i = 0; i < data.history.length; i++) {
          if (data.history[i] == 0) {
            history.push("Registered");
          } else if (data.history[i] == 1) {
            history.push("Cast for Nikki Haley");
          } else if (data.history[i] == 2) {
            history.push("Cast for Kamala Harris");
          } else {
            history.push("Invalid vote '"+data.history[i]+"'");
          }
        }
        data.history = history;
        ballotStatus = ok + '<br><code>' + JSON.stringify(data, null, 2) + '</code>';
        document.getElementById("random-ballot-result").innerHTML = ballotStatus;
        verifyRandomVoteRunning = false;
      });
    } else {
      r.text().then(rText => {
        console.log("got not-ok response when fetching random ballot", rText)
      });
      verifyRandomVoteRunning = false;
    }
  }).catch(err => {
    console.log("error fetching random ballot", err);
    verifyRandomVoteRunning = false;
  });
}

// shows notification bar with the specified message and level
const showNotification = async (msg, level) => {
  const notifBar = document.getElementById("notification-bar");
  const notifMsg = document.getElementById("notification-message");
  notifMsg.innerText = msg;
  notifBar.classList.add(level);
  notifBar.classList.remove("hidden");
}
// hides and resets the notification bar
const hideNotification = async () => {
  const notifBar = document.getElementById("notification-bar");
  const notifMsg = document.getElementById("notification-message");
  notifMsg.innerText = '';
  notifBar.classList.remove("error", "warning", "info");
  notifBar.classList.add("hidden");
}

// shows voting IDs in case the user has successfully registered
const showVoterAndBallotIDs = async (voterID, ballotID, tip) => {
  document.getElementById("voter-id").value = voterID;
  document.getElementById("ballot-id").value = ballotID;
  const voterAndBallotIDs = document.getElementById("voter-and-ballot-ids")
  voterAndBallotIDs.classList.remove("hidden");
  if (tip) {
    voterAndBallotIDs.setAttribute("open","");
  } else {
    document.getElementById("voting-tips").classList.add("hidden");
  }
}

// setupUI shows/hides page elements
const setupUI = async () => {
  const voterAndBallotIDsJSON = localStorage.getItem("immuvotingVoter");
  if (voterAndBallotIDsJSON) {
    const voterAndBallotIDs = JSON.parse(voterAndBallotIDsJSON);
    showVoterAndBallotIDs(voterAndBallotIDs.voter_id, voterAndBallotIDs.ballot_id, !voterAndBallotIDs.voted);
    if (!voterAndBallotIDs.voted) {
      document.getElementById("btn-vote-harris").classList.remove("hidden");
      document.getElementById("btn-vote-haley").classList.remove("hidden");
    }
  } else {
    document.getElementById("registration-panel").classList.remove("hidden");
  }
}

// registers voter
var registerVoterRunning = false;
const registerVoter = () => {
  if (registerVoterRunning) {
    return
  }
  registerVoterRunning = true;
  hideNotification();
  fetch(serverURL + '/register-voter', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      citizen_id: document.getElementById("id-number-input").value,
      name: document.getElementById("name-input").value,
      address: document.getElementById("address-input").value,
      email: document.getElementById("email-input").value
    })
  }).then(response => {
    if (!response.ok) {
      response.text().then(responseText => {
        showNotification(responseText, "error");
      });
    } else {
      response.json().then(responseJSON => {
        localStorage.setItem("immuvotingVoter", JSON.stringify(responseJSON));
        document.getElementById("registration-panel").classList.add("hidden");
        showVoterAndBallotIDs(responseJSON.voter_id, responseJSON.ballot_id, true);
        document.getElementById("btn-vote-harris").classList.remove("hidden");
        document.getElementById("btn-vote-haley").classList.remove("hidden");
      });
    }
    registerVoterRunning = false;
  }).catch(err => {
    console.log(err);
    registerVoterRunning = false;
  });
}

// vote casts the ballot
var voteRunning = false;
const vote = (candidateID, candidate) => {
  if (voteRunning) {
    return
  }
  voteRunning = true;
  hideNotification();
  const voterID = document.getElementById("voter-id").value;
  const ballotID = document.getElementById("ballot-id").value;
  if (voterID == "" || ballotID == "") {
    showNotification("please specify both your Voter and Ballot IDs", "error");
    voteRunning = false;
    return
  }
  fetch(serverURL + '/vote', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      voter_id: voterID,
      ballot_id: ballotID,
      vote: candidateID
    })
  }).then(response => {
    if (!response.ok) {
      response.text().then(responseText => {
        showNotification(responseText, "error");
      });
    } else {
      const voterDetails = {
        voter_id: voterID,
        ballot_id: ballotID,
        voted: true
      }
      localStorage.setItem("immuvotingVoter", JSON.stringify(voterDetails));
      document.getElementById("btn-vote-harris").classList.add("hidden");
      document.getElementById("btn-vote-haley").classList.add("hidden");
      document.getElementById("voting-tips").classList.add("hidden");
      showNotification("Congratulations! You've successfully cast your ballot for "+candidate+"!", "info");
    }
    voteRunning = false;
  }).catch(err => {
    console.log(err);
    voteRunning = false;
  });
}

// setup user interactions
const setupUserActions = async () => {
  // notifications
  document.getElementById("close-notification-bar").addEventListener("click", e => {
    e.preventDefault();
    hideNotification();
  });
  // register voter
  document.getElementById("register-btn").addEventListener("click", e => {
    e.preventDefault();
    registerVoter();
  });
  // already registered
  document.getElementById("already-registered-btn").addEventListener("click", e => {
    e.preventDefault();
    document.getElementById('registration-panel').classList.add('hidden');
    document.getElementById('voter-and-ballot-ids').setAttribute('open', '');
    document.getElementById('voter-and-ballot-ids').classList.remove('hidden');
    document.getElementById('btn-vote-harris').classList.remove('hidden');
    document.getElementById('btn-vote-haley').classList.remove('hidden');
  });
  // vote
  document.getElementById("btn-vote-harris").addEventListener("click", e => {
    e.preventDefault();
    vote(2, "Kamala Harris");
  });
  document.getElementById("btn-vote-haley").addEventListener("click", e => {
    e.preventDefault();
    vote(1, "Nikki Haley");
  });
  // clean-up voter details from storage and reload the page
  document.getElementById("btn-clean-up-voter").addEventListener("click", e => {
    e.preventDefault();
    const r = confirm(
      "This will wipe your Ballot and Voter ID from your local storage.\n"+
      "Are you sure you want to continue?");
    if (r) {
      localStorage.removeItem("immuvotingVoter");
      location.reload();
    }
  });
}

// initialize everything
document.addEventListener("DOMContentLoaded", function() {
  (async function loadAndRunGoWasm() {
    const go = new Go();
    const response = await fetch("verifier.wasm");
    const buffer = await response.arrayBuffer();
    const result = await WebAssembly.instantiate(buffer, go.importObject);
    go.run(result.instance);

    setupUI();
    setupUserActions();

    updateStats();
    setInterval(updateStats, 10000);

    updateBallotStatus();
    setInterval(updateBallotStatus, 15000);

    setInterval(verifyConsistency, 5000);
    setInterval(verifyRandomVote, 8000);
  })()
});