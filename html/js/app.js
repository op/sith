var sith = angular.module('sith', ['ngRoute', 'sithCtrls']);

sith.config(['$routeProvider',
  function($routeProvider) {
    $routeProvider.
      when('/search', {
        templateUrl: '/tmpl/search.html',
        controller: 'SearchCtrl'
      }).
      when('/playlists', {
        templateUrl: '/tmpl/playlists.html',
        controller: 'PlaylistsCtrl'
      }).
      when('/user/:username/playlist/:playlistId', {
        templateUrl: '/tmpl/playlist.html',
        controller: 'PlaylistCtrl'
      }).
      otherwise({
        redirectTo: '/search'
      });
  }]);
