var ctrls = angular.module('sith.controllers', []);

// Player handles the playback controls.
ctrls.controller('sith.ctrl.player', function($scope, $http, $interval) {
  // TODO update from other events
  $scope.context = true;
  $scope.current = {name: "", duration: 0, image: "holder.js/60x60"}
  $scope.playing = false;
  $scope.progress = 20;

  // offset is the numbers of seconds into the current playing track.
  $scope.offset = 0.0;
  var updateProgress = function() {
      $scope.progress = $scope.offset / $scope.current.duration * 100.0;
  };
  var progress = $interval(function() {
    if ($scope.playing && $scope.current.duration > 0) {
      $scope.offset += 0.1;
      updateProgress();
    }
  }, 100);
  $scope.$on('$destroy', function() {
    $interval.cancel(progress);
  });

  $scope.$on('track-end', function() {
    $scope.current.name = "";
  });

  var playTokenSnackbar = null;
  $scope.$on('play-token-lost', function() {
    playTokenSnackbar = $.snackbar({
      content: "Music paused because another account is streaming music.",
      timeout: 0
    });
    $scope.playing = false;
  });

  $scope.$on('play-track', function(event, playing) {
    $scope.offset = 0;
    updateProgress(0);

    if (playTokenSnackbar) {
      playTokenSnackbar.snackbar("hide");
    }
    $scope.current.name = playing.track.name;
    $scope.current.duration = playing.track.duration;
    $scope.playing = true;
  });

  $scope.$on('play-track-failed', function() {
    $.snackbar({
      content: "Failed to play track.",
      timeout: 1000
    });
    $scope.playing = false;
  });

  $scope.play = function() {
    if (playTokenSnackbar) {
      playTokenSnackbar.snackbar("hide");
    }
    $http.get('/player/play?oauth_token=xxx');
    $scope.playing = true;
  };
  $scope.pause = function() {
    $http.get('/player/pause?oauth_token=xxx');
    $scope.playing = false;
  };
});

ctrls.controller('sith.ctrl.search', function ($scope, $state, $http) {
  $scope.$on('search', function(event, query) {
    // TODO move all http calls into separate module
    $http.get('/search?query=' + query + '&oauth_token=xxx').success(function(data) {
      $scope.search = data;
      $scope.query = query;
    });
  });
  $scope.load = function(context, index, uri) {
    // TODO url encode parameters
    // HACK the query parameter is already found in the uri
    console.log('loading search result', context, index, uri, $scope.query);
    $http.get('/player/load?ctx=' + context + '&index=' + index + '&uri=' + uri + '&query=' + $scope.query + '&oauth_token=xxx').success(function() {
      console.log('Successfully changed track to: %s', uri);
    });
  };
});

ctrls.controller('sith.ctrl.search2', function($scope, $rootScope) {
  // TODO is there a better way to pass data to main controller?
  $scope.execute = function() {
    // TODO ng-minlength should take care of this?
    if (this.query) {
      $rootScope.$broadcast('search', this.query);
    }
  };
});

ctrls.controller('sith.ctrl.playlists', ['$scope', '$http', function($scope, $http) {
  $http.get('/playlists?limit=6789').success(function(data) {
    $scope.playlists = data.playlists;
  });
}]);

ctrls.controller('sith.ctrl.playlist', ['$scope', '$http', '$state', function($scope, $http, $state) {
  // TODO url escape?
  var url = '/user/' + $state.params.user + '/playlist/' + $state.params.playlistId;
  $http.get(url + '?limit=6789').success(function(data) {
    $scope.playlist = data.playlist;
  });

  $scope.load = function(context, index, uri) {
    console.log('loading playlist result', context, index, uri);
    $http.get('/player/load?ctx=' + context + '&index=' + index + '&uri=' + uri + '&oauth_token=xxx').success(function() {
      console.log('Successfully changed track to: %s', uri);
    });
  };
}]);

// ctrls.controller('LogController', ['$scope', 'LogService', function($scope, LogService) {
//   // $scope.logs = log.logs;
//
//   $scope.$on('log', function(evt, log) {
//     // $scope.logs.push(log);
//   });
// }]);
