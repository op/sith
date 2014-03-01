var sithCtrls = angular.module('sithCtrls', []);

sithCtrls.controller('SearchCtrl', ['$scope', '$http', function($scope, $http) {
  $scope.search = function() {
    // TODO ng-minlength should take care of this?
    if (this.query) {
      // TODO move all http calls into separate module
      $http.get('/search?query=' + this.query + '&oauth_token=xxx').success(function(data) {
        $scope.artists = data.artists;
        $scope.albums = data.albums;
        $scope.tracks = data.tracks;
      });
    } else {
      $scope.artists = [];
      $scope.albums = [];
      $scope.tracks = [];
    }
  }
}]);

sithCtrls.controller('PlaylistsCtrl', ['$scope', '$http', function($scope, $http) {
  $http.get('/playlists?limit=6789').success(function(data) {
    $scope.playlists = data.playlists;
  });
}]);

sithCtrls.controller('PlaylistCtrl', ['$scope', '$http', '$routeParams', function($scope, $http, $routeParams) {
  // TODO url escape?
  var url = '/user/' + $routeParams.user + '/playlist/' + $routeParams.playlistId;
  $http.get(url + '?limit=6789').success(function(data) {
    $scope.playlist = data.playlist;
  });
}]);
