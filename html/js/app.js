// Server sent events to subscribe to and propagate to angular
serverEvents = [
  'connection-error',
  'connection-state',
  'event-test',
  'log',
  'logged-in',
  'logged-out',
  'play-token-lost',
  'play-track',
  'play-track-failed',
  'streaming-error',
  'track-end',
  'track-end',
  'user-message',
];

angular.module('sith', [
	'ui.router',
	'sith.controllers'
])

.run(
	function($rootScope, $state, $stateParams) {
		$rootScope.$state = $state;
		$rootScope.$stateParams = $stateParams;

    // User messages from acccess point
    $rootScope.messages = [];
    $rootScope.$on('user-message', function(event, message) {
      $rootScope.messages.push(message);
    });

    // Push all server sent events through the root scope
    // TODO move to own service or something?
    propagateServerEvent = function(message) {
      $rootScope.$apply(function() {
        data = angular.fromJson(message.data);
        $rootScope.$broadcast(message.type, data);
      });
    };
    var events = new EventSource('/events');
    for (i in serverEvents) {
      events.addEventListener(serverEvents[i], propagateServerEvent);
    }
})

.config(
  function($stateProvider, $urlRouterProvider) {
    $stateProvider
      .state('index', {
        url: "/",
        views: {
          "main": {
            templateUrl: "tmpl/index.html"
          },
          "navigation": {
            templateUrl: "tmpl/index.navigation.html"
          }
        }
      })
      .state('log', {
        url: "/log",
        views: {
          "main": {
            controller: 'LogController',
            templateUrl: "tmpl/log.html"
          },
          "navigation": {
            templateUrl: "tmpl/index.navigation.html"
          }
        }
      })
      .state('search', {
        url: "/search",
        views: {
          "main": {
            controller: 'sith.ctrl.search',
            templateUrl: "tmpl/search.html"
          },
          "navigation": {
            templateUrl: "tmpl/search.navigation.html"
          }
        }
      })
      .state('playlists', {
        url: "/playlists",
        views: {
          "main": {
            controller: 'sith.ctrl.playlists',
            templateUrl: "tmpl/playlists.html"
          },
          "navigation": {
            templateUrl: "tmpl/index.navigation.html"
          }
        }
      })
      .state('playlist', {
        url: "/user/{username:[^/]+}/playlist/{playlistId:[^/]+}",
        views: {
          "main": {
            controller: 'sith.ctrl.playlist',
            templateUrl: "tmpl/playlist.html"
          },
          "navigation": {
            templateUrl: "tmpl/index.navigation.html"
          }
        }
      })
})

.service('LogService', function($rootScope) {
  var logs = [];

  $rootScope.on('log', function(evt, log) {
    console.log('LOG', log);
    logs.push(log);
  });

  this.all = function() {
    return logs;
  };
})

.controller('LogController', function($scope) {
  // $scope.logs = log.logs;

  $scope.$on('log', function(evt, log) {
    console.log('eehr', log);
    // $scope.logs.push(log);
  });
});
