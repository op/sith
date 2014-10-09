var sith = angular.module('sith', ['ui.router', 'sith.controllers']);

sith.config(
  function($stateProvider, $urlRouterProvider) {
    $stateProvider
      .state('index', {
        url: "/",
        templateUrl: "tmpl/index.html"
      })
      .state('search', {
        url: "/search",
        templateUrl: "tmpl/search.html",
        controller: 'sith.ctrl.search'
      })
      .state('playlists', {
        url: "/playlists",
        templateUrl: "tmpl/playlists.html",
        controller: 'sith.ctrl.playlists'
      })
      .state('playlist', {
        url: "/user/{username:[^/]+}/playlist/{playlistId:[^/]+}",
        templateUrl: "tmpl/playlist.html",
        controller: 'sith.ctrl.playlist'
      })
});
