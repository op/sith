var searchControllers = angular.module('searchControllers', []);

searchControllers.controller('SearchCtrl', ['$scope', '$http', function($scope, $http) {
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
