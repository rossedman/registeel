// log environment variable for api
console.log(process.env.REGISTEEL_API)

// start vue app
const vm = new Vue({
  el: '#app',
  data: {
    deployments: []
  },
  mounted() {
    axios.get(process.env.REGISTEEL_API)
      .then(response => { this.deployments = response.data })
  }
});
