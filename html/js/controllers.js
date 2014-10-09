var ctrls = angular.module('sith.controllers', []);

// Player handles the playback controls.
ctrls.controller('sith.ctrl.player', ['$scope', '$http', function($scope, $http) {
  // TODO update from other events
  $scope.context = true;
  $scope.current = {name: "Tony Sly - AMY", image: "holder.js/60x60"}
  $scope.playing = false;

  $scope.play = function() {
    $http.get('/player/play?oauth_token=xxx');
    this.playing = true;
  };
  $scope.pause = function() {
    $http.get('/player/pause?oauth_token=xxx');
    this.playing = false;
  };
}]);

ctrls.controller('sith.ctrl.search', ['$scope', '$state', '$http', function ($scope, $state, $http) {
  $scope.execute = function() {
    // TODO ng-minlength should take care of this?
    if (this.query) {
      // TODO move all http calls into separate module
      $http.get('/search?query=' + this.query + '&oauth_token=xxx').success(function(data) {
        $scope.search = data;
      });
    }
  };
  $scope.load = function(context, index, uri) {
    // TODO url encode parameters
    $http.get('/player/load?ctx=' + context + '&index=' + index + '&uri=' + uri + '&query=' + this.query + '&oauth_token=xxx').success(function() {
      console.log('Successfully changed track to: %s', uri);
    });
  };
}]);

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

  $scope.load = function(playlistUri, index, uri) {
    $http.get('/player/load?ctx=' + playlistUri + '&index=' + index + '&uri=' + uri + '&oauth_token=xxx').success(function() {
      console.log('Successfully changed track to: %s', uri);
    });
  };
}]);

