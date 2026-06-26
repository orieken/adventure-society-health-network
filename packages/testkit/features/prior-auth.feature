Feature: Prior Authorization
  Scenario: A provider requests prior authorization for resurrection
    Given an adventurer with a Diamond-severity incident
    And the treating provider is "Temple of the Healer, Vitesse"
    When the temple submits a prior authorization request for "resurrection"
    Then a 278 transaction is created with status "Pending"
    And the Adventure Society responds with an authorization decision

