// Copyright 2015 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

function GreetingsCtrl($scope, $http) {
  $scope.greetings = [];
  $scope.running = true;
  var api;
 
  loadAPI(function() {
    api = gapi.client.greetings;
    // load the messages now and refresh every second after.
    $scope.refresh();
    window.setInterval($scope.refresh, 1000);
  });

  $scope.refresh = function() {
    api.list({'limit':100}).execute(function(res) {
      $scope.running = false;
      $scope.greetings = res.result.items;
      $scope.$apply();
   });
  };

 // Load the greetings API
  $scope.add = function() {
    $scope.running = true;
    api.add($scope.msg).execute(function(res) {
      $scope.greetings.unshift($scope.msg);
      $scope.msg = {'author': $scope.msg.author};
      $scope.running = false;
      $scope.$apply();
  });
  };
}

function loadAPI(then) {
  var script = document.createElement('script');
  script.type = 'text/javascript';

  window.onAPILoaded = function() {
    var rootpath = "//" + window.location.host + "/_ah/api";
    gapi.client.load('greetings', 'v1', then, rootpath);
    window.onAPILoaded = undefined; 
  }
  script.src = 'https://apis.google.com/js/client.js?onload=onAPILoaded';

  var head = document.getElementsByTagName('head')[0];
  head.appendChild(script);
}
